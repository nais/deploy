package deployer

import (
	"fmt"

	"github.com/nais/deploy/pkg/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ExitCode int

// Keep separate to avoid skewing exit codes
const (
	ExitSuccess ExitCode = iota
	ExitDeploymentFailure
	ExitDeploymentError
	ExitDeploymentInactive
	ExitNoDeployment
	ExitUnavailable
	ExitInvocationFailure
	ExitInternalError
	ExitTemplateError
	ExitTimeout
)

type Error struct {
	Code ExitCode
	Err  error
}

func (err *Error) Error() string {
	return err.Err.Error()
}

func Errorf(exitCode ExitCode, format string, args ...interface{}) *Error {
	return &Error{
		Code: exitCode,
		Err:  fmt.Errorf(format, args...),
	}
}

func ErrorWrap(exitCode ExitCode, err error) *Error {
	return &Error{
		Code: exitCode,
		Err:  err,
	}
}

func ErrorExitCode(err error) ExitCode {
	if err == nil {
		return ExitSuccess
	}
	e, ok := err.(*Error)
	if !ok {
		return ExitInternalError
	}
	return e.Code
}

func ErrorStatus(status *pb.DeploymentStatus) error {
	switch status.GetState() {
	default:
		return nil
	case pb.DeploymentState_error:
		return Errorf(ExitDeploymentError, "deployment system encountered an error")
	case pb.DeploymentState_failure:
		return Errorf(ExitDeploymentFailure, "deployment failed")
	case pb.DeploymentState_inactive:
		return Errorf(ExitDeploymentInactive, "deployment has been stopped")
	}
}

func formatGrpcError(err error) string {
	gerr, ok := status.FromError(err)
	if !ok {
		return err.Error()
	}
	return fmt.Sprintf("%s: %s", gerr.Code(), gerr.Message())
}

func grpcErrorCode(err error) codes.Code {
	gerr := status.Convert(err)
	if gerr == nil {
		return codes.OK
	}
	return gerr.Code()
}
