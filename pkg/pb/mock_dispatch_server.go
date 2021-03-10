// Code generated by mockery v2.6.0. DO NOT EDIT.

package pb

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockDispatchServer is an autogenerated mock type for the DispatchServer type
type MockDispatchServer struct {
	mock.Mock
}

// Deployments provides a mock function with given fields: _a0, _a1
func (_m *MockDispatchServer) Deployments(_a0 *GetDeploymentOpts, _a1 Dispatch_DeploymentsServer) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*GetDeploymentOpts, Dispatch_DeploymentsServer) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ReportStatus provides a mock function with given fields: _a0, _a1
func (_m *MockDispatchServer) ReportStatus(_a0 context.Context, _a1 *DeploymentStatus) (*ReportStatusOpts, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *ReportStatusOpts
	if rf, ok := ret.Get(0).(func(context.Context, *DeploymentStatus) *ReportStatusOpts); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*ReportStatusOpts)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *DeploymentStatus) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mustEmbedUnimplementedDispatchServer provides a mock function with given fields:
func (_m *MockDispatchServer) mustEmbedUnimplementedDispatchServer() {
	_m.Called()
}