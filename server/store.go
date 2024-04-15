package main

import (
	"database/sql"
	"fmt"

	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/mattermost/mattermost/server/public/model"
)

type builder interface {
	ToSql() (string, []interface{}, error)
}

func (p *Plugin) SetupDB() error {
	if p.pluginAPI.Store.DriverName() != model.DatabaseDriverPostgres {
		return errors.New("this plugin is only supported on postgres")
	}

	origDB, err := p.pluginAPI.Store.GetMasterDB()
	if err != nil {
		return err
	}
	p.db = sqlx.NewDb(origDB, p.pluginAPI.Store.DriverName())

	builder := sq.StatementBuilder.PlaceholderFormat(sq.Question)
	builder = builder.PlaceholderFormat(sq.Dollar)
	p.builder = builder

	return p.SetupTables()
}

func (p *Plugin) doQuery(dest interface{}, b builder) error {
	sqlString, args, err := b.ToSql()
	if err != nil {
		return fmt.Errorf("failed to build sql: %w", err)
	}

	sqlString = p.db.Rebind(sqlString)

	return sqlx.Select(p.db, dest, sqlString, args...)
}

func (p *Plugin) execBuilder(b builder) (sql.Result, error) {
	sqlString, args, err := b.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build sql: %w", err)
	}

	sqlString = p.db.Rebind(sqlString)

	return p.db.Exec(sqlString, args...)
}

func (p *Plugin) SetupTables() error {
	if _, err := p.db.Exec(`
		CREATE TABLE IF NOT EXISTS LLM_Threads (
			RootPostID TEXT NOT NULL REFERENCES Posts(ID) PRIMARY KEY,
			Title TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("can't create feeback table: %w", err)
	}

	return nil
}

func (p *Plugin) saveTitleAsync(threadID, title string) {
	go func() {
		if err := p.saveTitle(threadID, title); err != nil {
			p.API.LogError("failed to save title: " + err.Error())
		}
	}()
}

func (p *Plugin) saveTitle(threadID, title string) error {
	_, err := p.execBuilder(p.builder.Insert("LLM_Threads").
		Columns("RootPostID", "Title").
		Values(threadID, title).
		Suffix("ON CONFLICT (RootPostID) DO UPDATE SET Title = ?", title))
	return err
}

type AIThread struct {
	ID         string
	Message    string
	ChannelID  string
	Title      string
	ReplyCount int
	UpdateAt   int64
}

func (p *Plugin) getAIThreads(dmChannelIDs []string) ([]AIThread, error) {
	var posts []AIThread
	if err := p.doQuery(&posts, p.builder.
		Select(
			"p.Id",
			"p.Message",
			"p.ChannelID",
			"COALESCE(t.Title, '') as Title",
			"(SELECT COUNT(*) FROM Posts WHERE Posts.RootId = p.Id AND DeleteAt = 0) AS ReplyCount",
			"p.UpdateAt",
		).
		From("Posts as p").
		Where(sq.Eq{"ChannelID": dmChannelIDs}).
		Where(sq.Eq{"RootId": ""}).
		Where(sq.Eq{"DeleteAt": 0}).
		LeftJoin("LLM_Threads as t ON t.RootPostID = p.Id").
		OrderBy("CreateAt DESC").
		Limit(60).
		Offset(0),
	); err != nil {
		return nil, fmt.Errorf("failed to get posts for bot DM: %w", err)
	}

	return posts, nil
}
