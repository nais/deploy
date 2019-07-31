package types

type SourceFunc func(Request) (*Credentials, error)

type SinkFunc func(Request, Credentials) error

type SourceFuncs map[Source]SourceFunc

type SinkFuncs map[Sink]SinkFunc

