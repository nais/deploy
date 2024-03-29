// Code generated by mockery v2.33.2. DO NOT EDIT.

package pb

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockDeployServer is an autogenerated mock type for the DeployServer type
type MockDeployServer struct {
	mock.Mock
}

// Deploy provides a mock function with given fields: _a0, _a1
func (_m *MockDeployServer) Deploy(_a0 context.Context, _a1 *DeploymentRequest) (*DeploymentStatus, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *DeploymentStatus
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *DeploymentRequest) (*DeploymentStatus, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *DeploymentRequest) *DeploymentStatus); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*DeploymentStatus)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *DeploymentRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Status provides a mock function with given fields: _a0, _a1
func (_m *MockDeployServer) Status(_a0 *DeploymentRequest, _a1 Deploy_StatusServer) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*DeploymentRequest, Deploy_StatusServer) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// mustEmbedUnimplementedDeployServer provides a mock function with given fields:
func (_m *MockDeployServer) mustEmbedUnimplementedDeployServer() {
	_m.Called()
}

// NewMockDeployServer creates a new instance of MockDeployServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockDeployServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockDeployServer {
	mock := &MockDeployServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
