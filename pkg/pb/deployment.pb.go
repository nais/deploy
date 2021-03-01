// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0
// 	protoc        v3.14.0
// source: pkg/pb/deployment.proto

package pb

import (
	proto "github.com/golang/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	structpb "google.golang.org/protobuf/types/known/structpb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

type DeploymentState int32

const (
	DeploymentState_success     DeploymentState = 0
	DeploymentState_error       DeploymentState = 1
	DeploymentState_failure     DeploymentState = 2
	DeploymentState_inactive    DeploymentState = 3
	DeploymentState_in_progress DeploymentState = 4
	DeploymentState_queued      DeploymentState = 5
	DeploymentState_pending     DeploymentState = 6
)

// Enum value maps for DeploymentState.
var (
	DeploymentState_name = map[int32]string{
		0: "success",
		1: "error",
		2: "failure",
		3: "inactive",
		4: "in_progress",
		5: "queued",
		6: "pending",
	}
	DeploymentState_value = map[string]int32{
		"success":     0,
		"error":       1,
		"failure":     2,
		"inactive":    3,
		"in_progress": 4,
		"queued":      5,
		"pending":     6,
	}
)

func (x DeploymentState) Enum() *DeploymentState {
	p := new(DeploymentState)
	*p = x
	return p
}

func (x DeploymentState) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (DeploymentState) Descriptor() protoreflect.EnumDescriptor {
	return file_pkg_pb_deployment_proto_enumTypes[0].Descriptor()
}

func (DeploymentState) Type() protoreflect.EnumType {
	return &file_pkg_pb_deployment_proto_enumTypes[0]
}

func (x DeploymentState) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use DeploymentState.Descriptor instead.
func (DeploymentState) EnumDescriptor() ([]byte, []int) {
	return file_pkg_pb_deployment_proto_rawDescGZIP(), []int{0}
}

type GithubRepository struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Owner string `protobuf:"bytes,1,opt,name=owner,proto3" json:"owner,omitempty"`
	Name  string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *GithubRepository) Reset() {
	*x = GithubRepository{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_pb_deployment_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GithubRepository) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GithubRepository) ProtoMessage() {}

func (x *GithubRepository) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_pb_deployment_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GithubRepository.ProtoReflect.Descriptor instead.
func (*GithubRepository) Descriptor() ([]byte, []int) {
	return file_pkg_pb_deployment_proto_rawDescGZIP(), []int{0}
}

func (x *GithubRepository) GetOwner() string {
	if x != nil {
		return x.Owner
	}
	return ""
}

func (x *GithubRepository) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type Kubernetes struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Resources []*structpb.Struct `protobuf:"bytes,1,rep,name=resources,proto3" json:"resources,omitempty"`
}

func (x *Kubernetes) Reset() {
	*x = Kubernetes{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_pb_deployment_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Kubernetes) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Kubernetes) ProtoMessage() {}

func (x *Kubernetes) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_pb_deployment_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Kubernetes.ProtoReflect.Descriptor instead.
func (*Kubernetes) Descriptor() ([]byte, []int) {
	return file_pkg_pb_deployment_proto_rawDescGZIP(), []int{1}
}

func (x *Kubernetes) GetResources() []*structpb.Struct {
	if x != nil {
		return x.Resources
	}
	return nil
}

type DeploymentRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ID                string                 `protobuf:"bytes,1,opt,name=ID,proto3" json:"ID,omitempty"`
	Time              *timestamppb.Timestamp `protobuf:"bytes,2,opt,name=time,proto3" json:"time,omitempty"`
	Deadline          *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=deadline,proto3" json:"deadline,omitempty"`
	Cluster           string                 `protobuf:"bytes,4,opt,name=cluster,proto3" json:"cluster,omitempty"`
	Team              string                 `protobuf:"bytes,5,opt,name=team,proto3" json:"team,omitempty"`
	GitRefSha         string                 `protobuf:"bytes,6,opt,name=gitRefSha,proto3" json:"gitRefSha,omitempty"`
	Kubernetes        *Kubernetes            `protobuf:"bytes,7,opt,name=kubernetes,proto3" json:"kubernetes,omitempty"`
	Repository        *GithubRepository      `protobuf:"bytes,8,opt,name=repository,proto3" json:"repository,omitempty"`
	GithubEnvironment string                 `protobuf:"bytes,9,opt,name=GithubEnvironment,proto3" json:"GithubEnvironment,omitempty"`
}

func (x *DeploymentRequest) Reset() {
	*x = DeploymentRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_pb_deployment_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeploymentRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeploymentRequest) ProtoMessage() {}

func (x *DeploymentRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_pb_deployment_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeploymentRequest.ProtoReflect.Descriptor instead.
func (*DeploymentRequest) Descriptor() ([]byte, []int) {
	return file_pkg_pb_deployment_proto_rawDescGZIP(), []int{2}
}

func (x *DeploymentRequest) GetID() string {
	if x != nil {
		return x.ID
	}
	return ""
}

func (x *DeploymentRequest) GetTime() *timestamppb.Timestamp {
	if x != nil {
		return x.Time
	}
	return nil
}

func (x *DeploymentRequest) GetDeadline() *timestamppb.Timestamp {
	if x != nil {
		return x.Deadline
	}
	return nil
}

func (x *DeploymentRequest) GetCluster() string {
	if x != nil {
		return x.Cluster
	}
	return ""
}

func (x *DeploymentRequest) GetTeam() string {
	if x != nil {
		return x.Team
	}
	return ""
}

func (x *DeploymentRequest) GetGitRefSha() string {
	if x != nil {
		return x.GitRefSha
	}
	return ""
}

func (x *DeploymentRequest) GetKubernetes() *Kubernetes {
	if x != nil {
		return x.Kubernetes
	}
	return nil
}

func (x *DeploymentRequest) GetRepository() *GithubRepository {
	if x != nil {
		return x.Repository
	}
	return nil
}

func (x *DeploymentRequest) GetGithubEnvironment() string {
	if x != nil {
		return x.GithubEnvironment
	}
	return ""
}

type DeploymentStatus struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ID      string                 `protobuf:"bytes,1,opt,name=ID,proto3" json:"ID,omitempty"`
	Request *DeploymentRequest     `protobuf:"bytes,2,opt,name=request,proto3" json:"request,omitempty"`
	Time    *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=time,proto3" json:"time,omitempty"`
	State   DeploymentState        `protobuf:"varint,4,opt,name=state,proto3,enum=pb.DeploymentState" json:"state,omitempty"`
	Message string                 `protobuf:"bytes,5,opt,name=message,proto3" json:"message,omitempty"`
}

func (x *DeploymentStatus) Reset() {
	*x = DeploymentStatus{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_pb_deployment_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeploymentStatus) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeploymentStatus) ProtoMessage() {}

func (x *DeploymentStatus) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_pb_deployment_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeploymentStatus.ProtoReflect.Descriptor instead.
func (*DeploymentStatus) Descriptor() ([]byte, []int) {
	return file_pkg_pb_deployment_proto_rawDescGZIP(), []int{3}
}

func (x *DeploymentStatus) GetID() string {
	if x != nil {
		return x.ID
	}
	return ""
}

func (x *DeploymentStatus) GetRequest() *DeploymentRequest {
	if x != nil {
		return x.Request
	}
	return nil
}

func (x *DeploymentStatus) GetTime() *timestamppb.Timestamp {
	if x != nil {
		return x.Time
	}
	return nil
}

func (x *DeploymentStatus) GetState() DeploymentState {
	if x != nil {
		return x.State
	}
	return DeploymentState_success
}

func (x *DeploymentStatus) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

type GetDeploymentOpts struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Cluster string `protobuf:"bytes,1,opt,name=cluster,proto3" json:"cluster,omitempty"`
}

func (x *GetDeploymentOpts) Reset() {
	*x = GetDeploymentOpts{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_pb_deployment_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetDeploymentOpts) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetDeploymentOpts) ProtoMessage() {}

func (x *GetDeploymentOpts) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_pb_deployment_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetDeploymentOpts.ProtoReflect.Descriptor instead.
func (*GetDeploymentOpts) Descriptor() ([]byte, []int) {
	return file_pkg_pb_deployment_proto_rawDescGZIP(), []int{4}
}

func (x *GetDeploymentOpts) GetCluster() string {
	if x != nil {
		return x.Cluster
	}
	return ""
}

type ReportStatusOpts struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ReportStatusOpts) Reset() {
	*x = ReportStatusOpts{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_pb_deployment_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ReportStatusOpts) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ReportStatusOpts) ProtoMessage() {}

func (x *ReportStatusOpts) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_pb_deployment_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ReportStatusOpts.ProtoReflect.Descriptor instead.
func (*ReportStatusOpts) Descriptor() ([]byte, []int) {
	return file_pkg_pb_deployment_proto_rawDescGZIP(), []int{5}
}

var File_pkg_pb_deployment_proto protoreflect.FileDescriptor

var file_pkg_pb_deployment_proto_rawDesc = []byte{
	0x0a, 0x17, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x62, 0x2f, 0x64, 0x65, 0x70, 0x6c, 0x6f, 0x79, 0x6d,
	0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x02, 0x70, 0x62, 0x1a, 0x1f, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74,
	0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1c,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f,
	0x73, 0x74, 0x72, 0x75, 0x63, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x3c, 0x0a, 0x10,
	0x47, 0x69, 0x74, 0x68, 0x75, 0x62, 0x52, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79,
	0x12, 0x14, 0x0a, 0x05, 0x6f, 0x77, 0x6e, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x05, 0x6f, 0x77, 0x6e, 0x65, 0x72, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x22, 0x43, 0x0a, 0x0a, 0x4b, 0x75,
	0x62, 0x65, 0x72, 0x6e, 0x65, 0x74, 0x65, 0x73, 0x12, 0x35, 0x0a, 0x09, 0x72, 0x65, 0x73, 0x6f,
	0x75, 0x72, 0x63, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x53, 0x74,
	0x72, 0x75, 0x63, 0x74, 0x52, 0x09, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x73, 0x22,
	0xeb, 0x02, 0x0a, 0x11, 0x44, 0x65, 0x70, 0x6c, 0x6f, 0x79, 0x6d, 0x65, 0x6e, 0x74, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x02, 0x49, 0x44, 0x12, 0x2e, 0x0a, 0x04, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52,
	0x04, 0x74, 0x69, 0x6d, 0x65, 0x12, 0x36, 0x0a, 0x08, 0x64, 0x65, 0x61, 0x64, 0x6c, 0x69, 0x6e,
	0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74,
	0x61, 0x6d, 0x70, 0x52, 0x08, 0x64, 0x65, 0x61, 0x64, 0x6c, 0x69, 0x6e, 0x65, 0x12, 0x18, 0x0a,
	0x07, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07,
	0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x65, 0x61, 0x6d, 0x18,
	0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x65, 0x61, 0x6d, 0x12, 0x1c, 0x0a, 0x09, 0x67,
	0x69, 0x74, 0x52, 0x65, 0x66, 0x53, 0x68, 0x61, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09,
	0x67, 0x69, 0x74, 0x52, 0x65, 0x66, 0x53, 0x68, 0x61, 0x12, 0x2e, 0x0a, 0x0a, 0x6b, 0x75, 0x62,
	0x65, 0x72, 0x6e, 0x65, 0x74, 0x65, 0x73, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0e, 0x2e,
	0x70, 0x62, 0x2e, 0x4b, 0x75, 0x62, 0x65, 0x72, 0x6e, 0x65, 0x74, 0x65, 0x73, 0x52, 0x0a, 0x6b,
	0x75, 0x62, 0x65, 0x72, 0x6e, 0x65, 0x74, 0x65, 0x73, 0x12, 0x34, 0x0a, 0x0a, 0x72, 0x65, 0x70,
	0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x18, 0x08, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x14, 0x2e,
	0x70, 0x62, 0x2e, 0x47, 0x69, 0x74, 0x68, 0x75, 0x62, 0x52, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74,
	0x6f, 0x72, 0x79, 0x52, 0x0a, 0x72, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x12,
	0x2c, 0x0a, 0x11, 0x47, 0x69, 0x74, 0x68, 0x75, 0x62, 0x45, 0x6e, 0x76, 0x69, 0x72, 0x6f, 0x6e,
	0x6d, 0x65, 0x6e, 0x74, 0x18, 0x09, 0x20, 0x01, 0x28, 0x09, 0x52, 0x11, 0x47, 0x69, 0x74, 0x68,
	0x75, 0x62, 0x45, 0x6e, 0x76, 0x69, 0x72, 0x6f, 0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x22, 0xc8, 0x01,
	0x0a, 0x10, 0x44, 0x65, 0x70, 0x6c, 0x6f, 0x79, 0x6d, 0x65, 0x6e, 0x74, 0x53, 0x74, 0x61, 0x74,
	0x75, 0x73, 0x12, 0x0e, 0x0a, 0x02, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02,
	0x49, 0x44, 0x12, 0x2f, 0x0a, 0x07, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x15, 0x2e, 0x70, 0x62, 0x2e, 0x44, 0x65, 0x70, 0x6c, 0x6f, 0x79, 0x6d,
	0x65, 0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x52, 0x07, 0x72, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x2e, 0x0a, 0x04, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x04, 0x74,
	0x69, 0x6d, 0x65, 0x12, 0x29, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x13, 0x2e, 0x70, 0x62, 0x2e, 0x44, 0x65, 0x70, 0x6c, 0x6f, 0x79, 0x6d, 0x65,
	0x6e, 0x74, 0x53, 0x74, 0x61, 0x74, 0x65, 0x52, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x12, 0x18,
	0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x22, 0x2d, 0x0a, 0x11, 0x47, 0x65, 0x74, 0x44,
	0x65, 0x70, 0x6c, 0x6f, 0x79, 0x6d, 0x65, 0x6e, 0x74, 0x4f, 0x70, 0x74, 0x73, 0x12, 0x18, 0x0a,
	0x07, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07,
	0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x22, 0x12, 0x0a, 0x10, 0x52, 0x65, 0x70, 0x6f, 0x72,
	0x74, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x4f, 0x70, 0x74, 0x73, 0x2a, 0x6e, 0x0a, 0x0f, 0x44,
	0x65, 0x70, 0x6c, 0x6f, 0x79, 0x6d, 0x65, 0x6e, 0x74, 0x53, 0x74, 0x61, 0x74, 0x65, 0x12, 0x0b,
	0x0a, 0x07, 0x73, 0x75, 0x63, 0x63, 0x65, 0x73, 0x73, 0x10, 0x00, 0x12, 0x09, 0x0a, 0x05, 0x65,
	0x72, 0x72, 0x6f, 0x72, 0x10, 0x01, 0x12, 0x0b, 0x0a, 0x07, 0x66, 0x61, 0x69, 0x6c, 0x75, 0x72,
	0x65, 0x10, 0x02, 0x12, 0x0c, 0x0a, 0x08, 0x69, 0x6e, 0x61, 0x63, 0x74, 0x69, 0x76, 0x65, 0x10,
	0x03, 0x12, 0x0f, 0x0a, 0x0b, 0x69, 0x6e, 0x5f, 0x70, 0x72, 0x6f, 0x67, 0x72, 0x65, 0x73, 0x73,
	0x10, 0x04, 0x12, 0x0a, 0x0a, 0x06, 0x71, 0x75, 0x65, 0x75, 0x65, 0x64, 0x10, 0x05, 0x12, 0x0b,
	0x0a, 0x07, 0x70, 0x65, 0x6e, 0x64, 0x69, 0x6e, 0x67, 0x10, 0x06, 0x32, 0xfb, 0x01, 0x0a, 0x06,
	0x44, 0x65, 0x70, 0x6c, 0x6f, 0x79, 0x12, 0x3f, 0x0a, 0x0b, 0x44, 0x65, 0x70, 0x6c, 0x6f, 0x79,
	0x6d, 0x65, 0x6e, 0x74, 0x73, 0x12, 0x15, 0x2e, 0x70, 0x62, 0x2e, 0x47, 0x65, 0x74, 0x44, 0x65,
	0x70, 0x6c, 0x6f, 0x79, 0x6d, 0x65, 0x6e, 0x74, 0x4f, 0x70, 0x74, 0x73, 0x1a, 0x15, 0x2e, 0x70,
	0x62, 0x2e, 0x44, 0x65, 0x70, 0x6c, 0x6f, 0x79, 0x6d, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x22, 0x00, 0x30, 0x01, 0x12, 0x3c, 0x0a, 0x0c, 0x52, 0x65, 0x70, 0x6f, 0x72,
	0x74, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x14, 0x2e, 0x70, 0x62, 0x2e, 0x44, 0x65, 0x70,
	0x6c, 0x6f, 0x79, 0x6d, 0x65, 0x6e, 0x74, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x1a, 0x14, 0x2e,
	0x70, 0x62, 0x2e, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x4f,
	0x70, 0x74, 0x73, 0x22, 0x00, 0x12, 0x37, 0x0a, 0x06, 0x44, 0x65, 0x70, 0x6c, 0x6f, 0x79, 0x12,
	0x15, 0x2e, 0x70, 0x62, 0x2e, 0x44, 0x65, 0x70, 0x6c, 0x6f, 0x79, 0x6d, 0x65, 0x6e, 0x74, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x14, 0x2e, 0x70, 0x62, 0x2e, 0x44, 0x65, 0x70, 0x6c,
	0x6f, 0x79, 0x6d, 0x65, 0x6e, 0x74, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x22, 0x00, 0x12, 0x39,
	0x0a, 0x06, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x15, 0x2e, 0x70, 0x62, 0x2e, 0x44, 0x65,
	0x70, 0x6c, 0x6f, 0x79, 0x6d, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x14, 0x2e, 0x70, 0x62, 0x2e, 0x44, 0x65, 0x70, 0x6c, 0x6f, 0x79, 0x6d, 0x65, 0x6e, 0x74, 0x53,
	0x74, 0x61, 0x74, 0x75, 0x73, 0x22, 0x00, 0x30, 0x01, 0x42, 0x39, 0x0a, 0x18, 0x6e, 0x6f, 0x2e,
	0x6e, 0x61, 0x76, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x73, 0x2e, 0x64, 0x65, 0x70, 0x6c, 0x6f,
	0x79, 0x6d, 0x65, 0x6e, 0x74, 0x5a, 0x1d, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x6e, 0x61, 0x69, 0x73, 0x2f, 0x64, 0x65, 0x70, 0x6c, 0x6f, 0x79, 0x2f, 0x70, 0x6b,
	0x67, 0x2f, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pkg_pb_deployment_proto_rawDescOnce sync.Once
	file_pkg_pb_deployment_proto_rawDescData = file_pkg_pb_deployment_proto_rawDesc
)

func file_pkg_pb_deployment_proto_rawDescGZIP() []byte {
	file_pkg_pb_deployment_proto_rawDescOnce.Do(func() {
		file_pkg_pb_deployment_proto_rawDescData = protoimpl.X.CompressGZIP(file_pkg_pb_deployment_proto_rawDescData)
	})
	return file_pkg_pb_deployment_proto_rawDescData
}

var file_pkg_pb_deployment_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_pkg_pb_deployment_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_pkg_pb_deployment_proto_goTypes = []interface{}{
	(DeploymentState)(0),          // 0: pb.DeploymentState
	(*GithubRepository)(nil),      // 1: pb.GithubRepository
	(*Kubernetes)(nil),            // 2: pb.Kubernetes
	(*DeploymentRequest)(nil),     // 3: pb.DeploymentRequest
	(*DeploymentStatus)(nil),      // 4: pb.DeploymentStatus
	(*GetDeploymentOpts)(nil),     // 5: pb.GetDeploymentOpts
	(*ReportStatusOpts)(nil),      // 6: pb.ReportStatusOpts
	(*structpb.Struct)(nil),       // 7: google.protobuf.Struct
	(*timestamppb.Timestamp)(nil), // 8: google.protobuf.Timestamp
}
var file_pkg_pb_deployment_proto_depIdxs = []int32{
	7,  // 0: pb.Kubernetes.resources:type_name -> google.protobuf.Struct
	8,  // 1: pb.DeploymentRequest.time:type_name -> google.protobuf.Timestamp
	8,  // 2: pb.DeploymentRequest.deadline:type_name -> google.protobuf.Timestamp
	2,  // 3: pb.DeploymentRequest.kubernetes:type_name -> pb.Kubernetes
	1,  // 4: pb.DeploymentRequest.repository:type_name -> pb.GithubRepository
	3,  // 5: pb.DeploymentStatus.request:type_name -> pb.DeploymentRequest
	8,  // 6: pb.DeploymentStatus.time:type_name -> google.protobuf.Timestamp
	0,  // 7: pb.DeploymentStatus.state:type_name -> pb.DeploymentState
	5,  // 8: pb.Deploy.Deployments:input_type -> pb.GetDeploymentOpts
	4,  // 9: pb.Deploy.ReportStatus:input_type -> pb.DeploymentStatus
	3,  // 10: pb.Deploy.Deploy:input_type -> pb.DeploymentRequest
	3,  // 11: pb.Deploy.Status:input_type -> pb.DeploymentRequest
	3,  // 12: pb.Deploy.Deployments:output_type -> pb.DeploymentRequest
	6,  // 13: pb.Deploy.ReportStatus:output_type -> pb.ReportStatusOpts
	4,  // 14: pb.Deploy.Deploy:output_type -> pb.DeploymentStatus
	4,  // 15: pb.Deploy.Status:output_type -> pb.DeploymentStatus
	12, // [12:16] is the sub-list for method output_type
	8,  // [8:12] is the sub-list for method input_type
	8,  // [8:8] is the sub-list for extension type_name
	8,  // [8:8] is the sub-list for extension extendee
	0,  // [0:8] is the sub-list for field type_name
}

func init() { file_pkg_pb_deployment_proto_init() }
func file_pkg_pb_deployment_proto_init() {
	if File_pkg_pb_deployment_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pkg_pb_deployment_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GithubRepository); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_pb_deployment_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Kubernetes); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_pb_deployment_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DeploymentRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_pb_deployment_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DeploymentStatus); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_pb_deployment_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetDeploymentOpts); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_pb_deployment_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ReportStatusOpts); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pkg_pb_deployment_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_pkg_pb_deployment_proto_goTypes,
		DependencyIndexes: file_pkg_pb_deployment_proto_depIdxs,
		EnumInfos:         file_pkg_pb_deployment_proto_enumTypes,
		MessageInfos:      file_pkg_pb_deployment_proto_msgTypes,
	}.Build()
	File_pkg_pb_deployment_proto = out.File
	file_pkg_pb_deployment_proto_rawDesc = nil
	file_pkg_pb_deployment_proto_goTypes = nil
	file_pkg_pb_deployment_proto_depIdxs = nil
}
