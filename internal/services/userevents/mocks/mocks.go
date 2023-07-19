// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/ProtonMail/proton-bridge/v3/internal/services/userevents (interfaces: EventSource,EventIDStore)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	proton "github.com/ProtonMail/go-proton-api"
	gomock "github.com/golang/mock/gomock"
)

// MockEventSource is a mock of EventSource interface.
type MockEventSource struct {
	ctrl     *gomock.Controller
	recorder *MockEventSourceMockRecorder
}

// MockEventSourceMockRecorder is the mock recorder for MockEventSource.
type MockEventSourceMockRecorder struct {
	mock *MockEventSource
}

// NewMockEventSource creates a new mock instance.
func NewMockEventSource(ctrl *gomock.Controller) *MockEventSource {
	mock := &MockEventSource{ctrl: ctrl}
	mock.recorder = &MockEventSourceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockEventSource) EXPECT() *MockEventSourceMockRecorder {
	return m.recorder
}

// GetEvent mocks base method.
func (m *MockEventSource) GetEvent(arg0 context.Context, arg1 string) ([]proton.Event, bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetEvent", arg0, arg1)
	ret0, _ := ret[0].([]proton.Event)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetEvent indicates an expected call of GetEvent.
func (mr *MockEventSourceMockRecorder) GetEvent(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetEvent", reflect.TypeOf((*MockEventSource)(nil).GetEvent), arg0, arg1)
}

// GetLatestEventID mocks base method.
func (m *MockEventSource) GetLatestEventID(arg0 context.Context) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLatestEventID", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLatestEventID indicates an expected call of GetLatestEventID.
func (mr *MockEventSourceMockRecorder) GetLatestEventID(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLatestEventID", reflect.TypeOf((*MockEventSource)(nil).GetLatestEventID), arg0)
}

// MockEventIDStore is a mock of EventIDStore interface.
type MockEventIDStore struct {
	ctrl     *gomock.Controller
	recorder *MockEventIDStoreMockRecorder
}

// MockEventIDStoreMockRecorder is the mock recorder for MockEventIDStore.
type MockEventIDStoreMockRecorder struct {
	mock *MockEventIDStore
}

// NewMockEventIDStore creates a new mock instance.
func NewMockEventIDStore(ctrl *gomock.Controller) *MockEventIDStore {
	mock := &MockEventIDStore{ctrl: ctrl}
	mock.recorder = &MockEventIDStoreMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockEventIDStore) EXPECT() *MockEventIDStoreMockRecorder {
	return m.recorder
}

// Load mocks base method.
func (m *MockEventIDStore) Load(arg0 context.Context) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Load", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Load indicates an expected call of Load.
func (mr *MockEventIDStoreMockRecorder) Load(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Load", reflect.TypeOf((*MockEventIDStore)(nil).Load), arg0)
}

// Store mocks base method.
func (m *MockEventIDStore) Store(arg0 context.Context, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Store", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Store indicates an expected call of Store.
func (mr *MockEventIDStoreMockRecorder) Store(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Store", reflect.TypeOf((*MockEventIDStore)(nil).Store), arg0, arg1)
}
