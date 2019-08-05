package types

type SourceFunc func(TokenIssuerRequest) (*Credentials, error)

type SinkFunc func(TokenIssuerRequest, Credentials) error

type SourceFuncs map[Source]SourceFunc

type SinkFuncs map[Sink]SinkFunc

