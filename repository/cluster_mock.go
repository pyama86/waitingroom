// Code generated by MockGen. DO NOT EDIT.
// Source: ./repository/cluster.go
//
// Generated by this command:
//
//	mockgen -package=repository -source=./repository/cluster.go -destination=./repository/cluster_mock.go ClusterRepositoryer
//

// Package repository is a generated GoMock package.
package repository

import (
	context "context"
	reflect "reflect"
	time "time"

	gomock "go.uber.org/mock/gomock"
)

// MockClusterRepositoryer is a mock of ClusterRepositoryer interface.
type MockClusterRepositoryer struct {
	ctrl     *gomock.Controller
	recorder *MockClusterRepositoryerMockRecorder
	isgomock struct{}
}

// MockClusterRepositoryerMockRecorder is the mock recorder for MockClusterRepositoryer.
type MockClusterRepositoryerMockRecorder struct {
	mock *MockClusterRepositoryer
}

// NewMockClusterRepositoryer creates a new mock instance.
func NewMockClusterRepositoryer(ctrl *gomock.Controller) *MockClusterRepositoryer {
	mock := &MockClusterRepositoryer{ctrl: ctrl}
	mock.recorder = &MockClusterRepositoryerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClusterRepositoryer) EXPECT() *MockClusterRepositoryerMockRecorder {
	return m.recorder
}

// GetLockforPermittedNumber mocks base method.
func (m *MockClusterRepositoryer) GetLockforPermittedNumber(arg0 context.Context, arg1 string, arg2 time.Duration) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLockforPermittedNumber", arg0, arg1, arg2)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLockforPermittedNumber indicates an expected call of GetLockforPermittedNumber.
func (mr *MockClusterRepositoryerMockRecorder) GetLockforPermittedNumber(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLockforPermittedNumber", reflect.TypeOf((*MockClusterRepositoryer)(nil).GetLockforPermittedNumber), arg0, arg1, arg2)
}
