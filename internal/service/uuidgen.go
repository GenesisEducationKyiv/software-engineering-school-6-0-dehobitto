package service

import "github.com/google/uuid"

// UUIDGenerator abstracts UUID generation so it can be replaced in tests.
type UUIDGenerator interface {
	New() string
}

type realUUIDGenerator struct{}

func (realUUIDGenerator) New() string { return uuid.New().String() }

// RealUUIDGenerator is the production implementation.
var RealUUIDGenerator UUIDGenerator = realUUIDGenerator{}
