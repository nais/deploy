// Code generated by mockery v2.20.0. DO NOT EDIT.

package pb

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"
)

// MockDispatchClient is an autogenerated mock type for the DispatchClient type
type MockDispatchClient struct {
	mock.Mock
}

// Deployments provides a mock function with given fields: ctx, in, opts
func (_m *MockDispatchClient) Deployments(ctx context.Context, in *GetDeploymentOpts, opts ...grpc.CallOption) (Dispatch_DeploymentsClient, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 Dispatch_DeploymentsClient
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *GetDeploymentOpts, ...grpc.CallOption) (Dispatch_DeploymentsClient, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *GetDeploymentOpts, ...grpc.CallOption) Dispatch_DeploymentsClient); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(Dispatch_DeploymentsClient)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *GetDeploymentOpts, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReportStatus provides a mock function with given fields: ctx, in, opts
func (_m *MockDispatchClient) ReportStatus(ctx context.Context, in *DeploymentStatus, opts ...grpc.CallOption) (*ReportStatusOpts, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *ReportStatusOpts
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *DeploymentStatus, ...grpc.CallOption) (*ReportStatusOpts, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *DeploymentStatus, ...grpc.CallOption) *ReportStatusOpts); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*ReportStatusOpts)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *DeploymentStatus, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewMockDispatchClient interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockDispatchClient creates a new instance of MockDispatchClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockDispatchClient(t mockConstructorTestingTNewMockDispatchClient) *MockDispatchClient {
	mock := &MockDispatchClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
