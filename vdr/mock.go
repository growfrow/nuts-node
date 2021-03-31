// Code generated by MockGen. DO NOT EDIT.
// Source: vdr/resolvers.go

// Package vdr is a generated GoMock package.
package vdr

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	did "github.com/nuts-foundation/go-did/did"
)

// MockNameResolver is a mock of NameResolver interface.
type MockNameResolver struct {
	ctrl     *gomock.Controller
	recorder *MockNameResolverMockRecorder
}

// MockNameResolverMockRecorder is the mock recorder for MockNameResolver.
type MockNameResolverMockRecorder struct {
	mock *MockNameResolver
}

// NewMockNameResolver creates a new mock instance.
func NewMockNameResolver(ctrl *gomock.Controller) *MockNameResolver {
	mock := &MockNameResolver{ctrl: ctrl}
	mock.recorder = &MockNameResolverMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockNameResolver) EXPECT() *MockNameResolverMockRecorder {
	return m.recorder
}

// Resolve mocks base method.
func (m *MockNameResolver) Resolve(input did.DID) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Resolve", input)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Resolve indicates an expected call of Resolve.
func (mr *MockNameResolverMockRecorder) Resolve(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Resolve", reflect.TypeOf((*MockNameResolver)(nil).Resolve), input)
}
