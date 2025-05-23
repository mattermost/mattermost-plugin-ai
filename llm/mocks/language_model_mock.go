// Code generated by mockery; DO NOT EDIT.
// github.com/vektra/mockery
// template: testify

package mocks

import (
	"github.com/mattermost/mattermost-plugin-ai/llm"
	mock "github.com/stretchr/testify/mock"
)

// NewMockLanguageModel creates a new instance of MockLanguageModel. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockLanguageModel(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockLanguageModel {
	mock := &MockLanguageModel{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// MockLanguageModel is an autogenerated mock type for the LanguageModel type
type MockLanguageModel struct {
	mock.Mock
}

type MockLanguageModel_Expecter struct {
	mock *mock.Mock
}

func (_m *MockLanguageModel) EXPECT() *MockLanguageModel_Expecter {
	return &MockLanguageModel_Expecter{mock: &_m.Mock}
}

// ChatCompletion provides a mock function for the type MockLanguageModel
func (_mock *MockLanguageModel) ChatCompletion(conversation llm.CompletionRequest, opts ...llm.LanguageModelOption) (*llm.TextStreamResult, error) {
	var tmpRet mock.Arguments
	if len(opts) > 0 {
		tmpRet = _mock.Called(conversation, opts)
	} else {
		tmpRet = _mock.Called(conversation)
	}
	ret := tmpRet

	if len(ret) == 0 {
		panic("no return value specified for ChatCompletion")
	}

	var r0 *llm.TextStreamResult
	var r1 error
	if returnFunc, ok := ret.Get(0).(func(llm.CompletionRequest, ...llm.LanguageModelOption) (*llm.TextStreamResult, error)); ok {
		return returnFunc(conversation, opts...)
	}
	if returnFunc, ok := ret.Get(0).(func(llm.CompletionRequest, ...llm.LanguageModelOption) *llm.TextStreamResult); ok {
		r0 = returnFunc(conversation, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*llm.TextStreamResult)
		}
	}
	if returnFunc, ok := ret.Get(1).(func(llm.CompletionRequest, ...llm.LanguageModelOption) error); ok {
		r1 = returnFunc(conversation, opts...)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

// MockLanguageModel_ChatCompletion_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ChatCompletion'
type MockLanguageModel_ChatCompletion_Call struct {
	*mock.Call
}

// ChatCompletion is a helper method to define mock.On call
//   - conversation
//   - opts
func (_e *MockLanguageModel_Expecter) ChatCompletion(conversation interface{}, opts ...interface{}) *MockLanguageModel_ChatCompletion_Call {
	return &MockLanguageModel_ChatCompletion_Call{Call: _e.mock.On("ChatCompletion",
		append([]interface{}{conversation}, opts...)...)}
}

func (_c *MockLanguageModel_ChatCompletion_Call) Run(run func(conversation llm.CompletionRequest, opts ...llm.LanguageModelOption)) *MockLanguageModel_ChatCompletion_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := args[1].([]llm.LanguageModelOption)
		run(args[0].(llm.CompletionRequest), variadicArgs...)
	})
	return _c
}

func (_c *MockLanguageModel_ChatCompletion_Call) Return(textStreamResult *llm.TextStreamResult, err error) *MockLanguageModel_ChatCompletion_Call {
	_c.Call.Return(textStreamResult, err)
	return _c
}

func (_c *MockLanguageModel_ChatCompletion_Call) RunAndReturn(run func(conversation llm.CompletionRequest, opts ...llm.LanguageModelOption) (*llm.TextStreamResult, error)) *MockLanguageModel_ChatCompletion_Call {
	_c.Call.Return(run)
	return _c
}

// ChatCompletionNoStream provides a mock function for the type MockLanguageModel
func (_mock *MockLanguageModel) ChatCompletionNoStream(conversation llm.CompletionRequest, opts ...llm.LanguageModelOption) (string, error) {
	var tmpRet mock.Arguments
	if len(opts) > 0 {
		tmpRet = _mock.Called(conversation, opts)
	} else {
		tmpRet = _mock.Called(conversation)
	}
	ret := tmpRet

	if len(ret) == 0 {
		panic("no return value specified for ChatCompletionNoStream")
	}

	var r0 string
	var r1 error
	if returnFunc, ok := ret.Get(0).(func(llm.CompletionRequest, ...llm.LanguageModelOption) (string, error)); ok {
		return returnFunc(conversation, opts...)
	}
	if returnFunc, ok := ret.Get(0).(func(llm.CompletionRequest, ...llm.LanguageModelOption) string); ok {
		r0 = returnFunc(conversation, opts...)
	} else {
		r0 = ret.Get(0).(string)
	}
	if returnFunc, ok := ret.Get(1).(func(llm.CompletionRequest, ...llm.LanguageModelOption) error); ok {
		r1 = returnFunc(conversation, opts...)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

// MockLanguageModel_ChatCompletionNoStream_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ChatCompletionNoStream'
type MockLanguageModel_ChatCompletionNoStream_Call struct {
	*mock.Call
}

// ChatCompletionNoStream is a helper method to define mock.On call
//   - conversation
//   - opts
func (_e *MockLanguageModel_Expecter) ChatCompletionNoStream(conversation interface{}, opts ...interface{}) *MockLanguageModel_ChatCompletionNoStream_Call {
	return &MockLanguageModel_ChatCompletionNoStream_Call{Call: _e.mock.On("ChatCompletionNoStream",
		append([]interface{}{conversation}, opts...)...)}
}

func (_c *MockLanguageModel_ChatCompletionNoStream_Call) Run(run func(conversation llm.CompletionRequest, opts ...llm.LanguageModelOption)) *MockLanguageModel_ChatCompletionNoStream_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := args[1].([]llm.LanguageModelOption)
		run(args[0].(llm.CompletionRequest), variadicArgs...)
	})
	return _c
}

func (_c *MockLanguageModel_ChatCompletionNoStream_Call) Return(s string, err error) *MockLanguageModel_ChatCompletionNoStream_Call {
	_c.Call.Return(s, err)
	return _c
}

func (_c *MockLanguageModel_ChatCompletionNoStream_Call) RunAndReturn(run func(conversation llm.CompletionRequest, opts ...llm.LanguageModelOption) (string, error)) *MockLanguageModel_ChatCompletionNoStream_Call {
	_c.Call.Return(run)
	return _c
}

// CountTokens provides a mock function for the type MockLanguageModel
func (_mock *MockLanguageModel) CountTokens(text string) int {
	ret := _mock.Called(text)

	if len(ret) == 0 {
		panic("no return value specified for CountTokens")
	}

	var r0 int
	if returnFunc, ok := ret.Get(0).(func(string) int); ok {
		r0 = returnFunc(text)
	} else {
		r0 = ret.Get(0).(int)
	}
	return r0
}

// MockLanguageModel_CountTokens_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CountTokens'
type MockLanguageModel_CountTokens_Call struct {
	*mock.Call
}

// CountTokens is a helper method to define mock.On call
//   - text
func (_e *MockLanguageModel_Expecter) CountTokens(text interface{}) *MockLanguageModel_CountTokens_Call {
	return &MockLanguageModel_CountTokens_Call{Call: _e.mock.On("CountTokens", text)}
}

func (_c *MockLanguageModel_CountTokens_Call) Run(run func(text string)) *MockLanguageModel_CountTokens_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MockLanguageModel_CountTokens_Call) Return(n int) *MockLanguageModel_CountTokens_Call {
	_c.Call.Return(n)
	return _c
}

func (_c *MockLanguageModel_CountTokens_Call) RunAndReturn(run func(text string) int) *MockLanguageModel_CountTokens_Call {
	_c.Call.Return(run)
	return _c
}

// InputTokenLimit provides a mock function for the type MockLanguageModel
func (_mock *MockLanguageModel) InputTokenLimit() int {
	ret := _mock.Called()

	if len(ret) == 0 {
		panic("no return value specified for InputTokenLimit")
	}

	var r0 int
	if returnFunc, ok := ret.Get(0).(func() int); ok {
		r0 = returnFunc()
	} else {
		r0 = ret.Get(0).(int)
	}
	return r0
}

// MockLanguageModel_InputTokenLimit_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'InputTokenLimit'
type MockLanguageModel_InputTokenLimit_Call struct {
	*mock.Call
}

// InputTokenLimit is a helper method to define mock.On call
func (_e *MockLanguageModel_Expecter) InputTokenLimit() *MockLanguageModel_InputTokenLimit_Call {
	return &MockLanguageModel_InputTokenLimit_Call{Call: _e.mock.On("InputTokenLimit")}
}

func (_c *MockLanguageModel_InputTokenLimit_Call) Run(run func()) *MockLanguageModel_InputTokenLimit_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockLanguageModel_InputTokenLimit_Call) Return(n int) *MockLanguageModel_InputTokenLimit_Call {
	_c.Call.Return(n)
	return _c
}

func (_c *MockLanguageModel_InputTokenLimit_Call) RunAndReturn(run func() int) *MockLanguageModel_InputTokenLimit_Call {
	_c.Call.Return(run)
	return _c
}
