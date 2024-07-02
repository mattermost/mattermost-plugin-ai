import React, { useEffect, useState, useCallback } from "react";
import styled from "styled-components";
import { FormattedMessage, useIntl } from "react-intl";

import { PlusIcon } from "@mattermost/compass-icons/components";

import { Pill } from "../pill";

import { setUserProfilePictureByUsername } from "@/client";

import { ServiceData } from "./service";
import Panel, { PanelFooterText } from "./panel";
import Bots, { firstNewBot } from "./bots";
import { LLMBotConfig } from "./bot";
import { ItemList, SelectionItem, SelectionItemOption } from "./item";
import NoBotsPage from "./no_bots_page";

import { ExternalTool } from "./external_tool";
import ExternalToolForm from "./tool_form";
import { TertiaryButton } from "../assets/buttons";

type Config = {
  services: ServiceData[];
  ExternalTools: ExternalTool[];
  llmBackend: string;
  bots: LLMBotConfig[];
  defaultBotName: string;
  transcriptBackend: string;
  enableLLMTrace: boolean;
  enableCallSummary: boolean;

  enableUserRestrictions: boolean;
  allowPrivateChannels: boolean;
  allowedTeamIds: string;
  onlyUsersOnTeam: string;
};

type Props = {
  id: string;
  label: string;
  helpText: React.ReactNode;
  value: Config;
  disabled: boolean;
  config: any;
  currentState: any;
  license: any;
  setByEnv: boolean;
  onChange: (id: string, value: any) => void;
  setSaveNeeded: () => void;
  registerSaveAction: (
    action: () => Promise<{ error?: { message?: string } }>
  ) => void;
  unRegisterSaveAction: (
    action: () => Promise<{ error?: { message?: string } }>
  ) => void;
};

const MessageContainer = styled.div`
  display: flex;
  align-items: center;
  flex-direction: row;
  gap: 5px;
  padding: 10px 12px;
  background: white;
  border-radius: 4px;
  border: 1px solid rgba(63, 67, 80, 0.08);
`;

const PanelContainer = styled.div`
  display: flex;
  flex-direction: column;
  padding: 32px;
  gap: 32px;
  border: 1px solid #ccc;
  background: white;
  border-radius: 4px;
  box-shadow: 0px 2px 3px 0px rgba(0, 0, 0, 0.08);
`;

const PanelHeader = styled.div`
  display: flex;
  flex-direction: column;
  gap: 4px;
`;

const PanelTitle = styled.div`
  font-size: 16px;
  font-weight: 600;
`;

const PanelSubtitle = styled.div`
  color: rgba(63, 67, 80, 0.72);
  font-size: 14px;
  font-weight: 400;
`;

const PlusAIServiceIcon = styled(PlusIcon)`
  width: 18px;
  height: 18px;
  margin-right: 8px;
`;

const ConfigContainer = styled.div`
  display: flex;
  flex-direction: column;
  gap: 20px;
`;

const EnterpriseChipContainer = styled.div`
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 8px;
`;

const defaultConfig = {
  services: [],
  ExternalTools: [],
  llmBackend: "",
  transcriptBackend: "",
  imageGeneratorBackend: "",
  enableLLMTrace: false,
  enableUserRestrictions: false,
  allowPrivateChannels: false,
  allowedTeamIds: "",
  onlyUsersOnTeam: "",
};

const BetaMessage = () => (
  <MessageContainer>
    <span>
      <FormattedMessage
        defaultMessage="To report a bug or to provide feedback, <link>create a new issue in the plugin repository</link>."
        values={{
          link: (chunks: any) => (
            <a
              target={"_blank"}
              rel={"noopener noreferrer"}
              href="http://github.com/mattermost/mattermost-plugin-ai/issues"
            >
              {chunks}
            </a>
          ),
        }}
      />
    </span>
  </MessageContainer>
);

const Config = (props: Props) => {
  const value = props.value || defaultConfig;
  const currentServices = value.services;
  const currentExternalTools = value.ExternalTools;
  const [avatarUpdates, setAvatarUpdates] = useState<{ [key: string]: File }>(
    {}
  );
  const intl = useIntl();

  useEffect(() => {
    const save = async () => {
      Object.keys(avatarUpdates).map((username: string) =>
        setUserProfilePictureByUsername(username, avatarUpdates[username])
      );
      return {};
    };
    props.registerSaveAction(save);
    return () => {
      props.unRegisterSaveAction(save);
    };
  }, [avatarUpdates]);

  const botChangedAvatar = (bot: LLMBotConfig, image: File) => {
    setAvatarUpdates((prev: { [key: string]: File }) => ({
      ...prev,
      [bot.name]: image,
    }));
    props.setSaveNeeded();
  };

  const addFirstBot = () => {
    const id = Math.random().toString(36).substring(2, 22);
    props.onChange(props.id, {
      ...value,
      bots: [
        {
          ...firstNewBot,
          id,
        },
      ],
    });
  };

  const addNewService = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      e.stopPropagation();
      const newService = {
        id: Math.random().toString(36).substring(2, 22),
        name: "AI Engine",
        serviceName: "openai",
        defaultModel: "",
        url: "",
        apiKey: "",
        orgId: "",
        username: "",
        password: "",
        tokenLimit: 0,
      };

      let counter = 1;
      for (;;) {
        let isNew = true;
        for (const service of currentServices) {
          if (service.name === newService.name) {
            isNew = false;
          }
        }
        if (isNew) {
          break;
        }
        newService.name = `AI Engine ${counter}`;
        counter++;
      }
      if (value.services.length === 0) {
        props.onChange(props.id, {
          ...value,
          services: [...currentServices, newService],
          llmBackend: newService.name,
          transcriptBackend: newService.name,
          imageGeneratorBackend: newService.name,
        });
      } else {
        props.onChange(props.id, {
          ...value,
          services: [...currentServices, newService],
        });
      }
    },
    [value, currentServices]
  );

  const addNewTool = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      e.stopPropagation();

      const newTool = {} as ExternalTool;

      if (value.ExternalTools.length === 0) {
        props.onChange(props.id, { ...value, ExternalTools: [newTool] });
      }
    },
    [value, currentExternalTools]
  );

  const deleteTool = (deletedTool: ExternalTool) => {
    props.onChange(props.id, { ...value, ExternalTools: [] });
    props.setSaveNeeded();
  };

  const changeTool = (changedTool: ExternalTool) => {
    props.onChange(props.id, { ...value, ExternalTools: [changedTool] });
    props.setSaveNeeded();
  };

  const deleteService = (deletedService: ServiceData) => {
    const updatedServiceIdx = currentServices.indexOf(deletedService);
    if (updatedServiceIdx === -1) {
      throw new Error("Service not found");
    }
    let newValue = value;
    if (currentServices.length > 1) {
      if (value.llmBackend === deletedService.name) {
        newValue = { ...newValue, llmBackend: value.services[0]?.name || "" };
      }
      if (value.transcriptBackend === deletedService.name) {
        newValue = {
          ...newValue,
          transcriptBackend: value.services[0]?.name || "",
        };
      }
    } else {
      newValue = {
        ...newValue,
        llmBackend: "",
        transcriptBackend: "",
      };
    }
    props.onChange(props.id, {
      ...newValue,
      services: [
        ...currentServices.slice(0, updatedServiceIdx),
        ...currentServices.slice(updatedServiceIdx + 1),
      ],
    });
    props.setSaveNeeded();
  };

  const changeService = (changedService: ServiceData) => {
    const updatedServiceIdx = currentServices.findIndex(
      (s) => changedService.id === s.id
    );
    if (updatedServiceIdx === -1) {
      throw new Error("Service not found");
    }
    let newValue = value;
    if (value.llmBackend === currentServices[updatedServiceIdx].name) {
      newValue = { ...newValue, llmBackend: changedService.name };
    }
    if (value.transcriptBackend === currentServices[updatedServiceIdx].name) {
      newValue = { ...newValue, transcriptBackend: changedService.name };
    }
    props.onChange(props.id, {
      ...newValue,
      services: [
        ...currentServices.slice(0, updatedServiceIdx),
        changedService,
        ...currentServices.slice(updatedServiceIdx + 1),
      ],
    });
    props.setSaveNeeded();
  };

  if (!props.value?.bots || props.value.bots.length === 0) {
    return (
      <ConfigContainer>
        <BetaMessage />
        <NoBotsPage onAddBotPressed={addFirstBot} />
      </ConfigContainer>
    );
  }

  return (
    <ConfigContainer>
      <BetaMessage />
      <Panel
        title={intl.formatMessage({ defaultMessage: "AI Bots" })}
        subtitle={intl.formatMessage({
          defaultMessage: "Multiple AI services can be configured below.",
        })}
      >
        <Bots
          bots={props.value.bots ?? []}
          onChange={(bots: LLMBotConfig[]) => {
            if (
              value.bots.findIndex(
                (bot) => bot.name === value.defaultBotName
              ) === -1
            ) {
              props.onChange(props.id, {
                ...value,
                bots,
                defaultBotName: bots[0].name,
              });
            } else {
              props.onChange(props.id, { ...value, bots });
            }
            props.setSaveNeeded();
          }}
          botChangedAvatar={botChangedAvatar}
        />
        <PanelFooterText>
          <FormattedMessage defaultMessage="AI services are third party services; Mattermost is not responsible for output." />
        </PanelFooterText>
      </Panel>
      <Panel
        title="External Tools"
        subtitle="Allow the AI Copilot to integrate with external workflow tools like N8N or Zapier"
      >
        {currentExternalTools.map((tool) => (
          <ExternalToolForm
            key={tool.Provider}
            externalTool={tool}
            onDelete={deleteTool}
            onChange={changeTool}
          />
        ))}
        <EnterpriseChipContainer>
          <TertiaryButton
            onClick={addNewTool}
            disabled={currentExternalTools.length >= 1}
          >
            <PlusAIServiceIcon />
            {"Add AI Tool Service"}
          </TertiaryButton>
        </EnterpriseChipContainer>
        <PanelFooterText>
          {
            "AI tools are third party services; Mattermost is not responsible for output."
          }
        </PanelFooterText>
      </Panel>
      <Panel
        title={intl.formatMessage({ defaultMessage: "AI Functions" })}
        subtitle={intl.formatMessage({
          defaultMessage:
            "Choose which bot you want to be the default for each function.",
        })}
      >
        <ItemList>
          <SelectionItem
            label={intl.formatMessage({ defaultMessage: "Default bot" })}
            value={value.defaultBotName}
            onChange={(e) => {
              props.onChange(props.id, {
                ...value,
                defaultBotName: e.target.value,
              });
              props.setSaveNeeded();
            }}
          >
            {props.value.bots.map((bot: LLMBotConfig) => (
              <SelectionItemOption key={bot.name} value={bot.name}>
                {bot.displayName}
              </SelectionItemOption>
            ))}
          </SelectionItem>
        </ItemList>
      </Panel>

      <Panel
        title={intl.formatMessage({
          defaultMessage: "User restrictions (experimental)",
        })}
        subtitle={intl.formatMessage({
          defaultMessage:
            "Enable restrictions to allow or not users to use AI in this instance.",
        })}
      >
        <div className="form-group">
          <label className="control-label col-sm-4">
            <FormattedMessage defaultMessage="Enable User Restrictions:" />
          </label>
          <div className="col-sm-8">
            <label className="radio-inline">
              <input
                type="radio"
                value="true"
                checked={value.enableUserRestrictions}
                onChange={() =>
                  props.onChange(props.id, {
                    ...value,
                    enableUserRestrictions: true,
                  })
                }
              />
              <span>
                <FormattedMessage defaultMessage="true" />
              </span>
            </label>
            <label className="radio-inline">
              <input
                type="radio"
                value="false"
                checked={!value.enableUserRestrictions}
                onChange={() =>
                  props.onChange(props.id, {
                    ...value,
                    enableUserRestrictions: false,
                  })
                }
              />
              <span>
                <FormattedMessage defaultMessage="false" />
              </span>
            </label>
            <div className="help-text">
              <span>
                <FormattedMessage defaultMessage="Global flag for all below settings." />
              </span>
            </div>
          </div>
        </div>
        {value.enableUserRestrictions && (
          <>
            <div className="form-group">
              <label className="control-label col-sm-4">
                <FormattedMessage defaultMessage="Allow Private Channels:" />
              </label>
              <div className="col-sm-8">
                <label className="radio-inline">
                  <input
                    type="radio"
                    value="true"
                    checked={value.allowPrivateChannels}
                    onChange={() =>
                      props.onChange(props.id, {
                        ...value,
                        allowPrivateChannels: true,
                      })
                    }
                  />
                  <span>
                    <FormattedMessage defaultMessage="true" />
                  </span>
                </label>
                <label className="radio-inline">
                  <input
                    type="radio"
                    value="false"
                    checked={!value.allowPrivateChannels}
                    onChange={() =>
                      props.onChange(props.id, {
                        ...value,
                        allowPrivateChannels: false,
                      })
                    }
                  />
                  <span>
                    <FormattedMessage defaultMessage="false" />
                  </span>
                </label>
              </div>
            </div>
            <div className="form-group">
              <label
                className="control-label col-sm-4"
                htmlFor="ai-allow-team-ids"
              >
                <FormattedMessage defaultMessage="Allow Team IDs (csv):" />
              </label>
              <div className="col-sm-8">
                <input
                  id="ai-allow-team-ids"
                  className="form-control"
                  type="text"
                  value={value.allowedTeamIds}
                  onChange={(e) =>
                    props.onChange(props.id, {
                      ...value,
                      allowedTeamIds: e.target.value,
                    })
                  }
                />
              </div>
            </div>
            <div className="form-group">
              <label
                className="control-label col-sm-4"
                htmlFor="ai-only-users-on-team"
              >
                <FormattedMessage defaultMessage="Only Users on Team:" />
              </label>
              <div className="col-sm-8">
                <input
                  id="ai-only-users-on-team"
                  className="form-control"
                  type="text"
                  value={value.onlyUsersOnTeam}
                  onChange={(e) =>
                    props.onChange(props.id, {
                      ...value,
                      onlyUsersOnTeam: e.target.value,
                    })
                  }
                />
              </div>
            </div>
          </>
        )}
      </Panel>

      <Panel
        title={intl.formatMessage({ defaultMessage: "Debug" })}
        subtitle=""
      >
        <div className="form-group">
          <label className="control-label col-sm-4" htmlFor="ai-service-name">
            <FormattedMessage defaultMessage="Enable LLM Trace:" />
          </label>
          <div className="col-sm-8">
            <label className="radio-inline">
              <input
                type="radio"
                value="true"
                checked={value.enableLLMTrace}
                onChange={() =>
                  props.onChange(props.id, { ...value, enableLLMTrace: true })
                }
              />
              <span>
                <FormattedMessage defaultMessage="true" />
              </span>
            </label>
            <label className="radio-inline">
              <input
                type="radio"
                value="false"
                checked={!value.enableLLMTrace}
                onChange={() =>
                  props.onChange(props.id, { ...value, enableLLMTrace: false })
                }
              />
              <span>
                <FormattedMessage defaultMessage="false" />
              </span>
            </label>
            <div className="help-text">
              <span>
                <FormattedMessage defaultMessage="Enable tracing of LLM requests. Outputs whole conversations to the logs." />
              </span>
            </div>
          </div>
        </div>
      </Panel>
    </ConfigContainer>
  );
};
export default Config;
