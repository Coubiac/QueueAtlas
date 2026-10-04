package file

import "errors"

type StartAt string

const (
	StartAtBeginning StartAt = "beginning"
	StartAtEnd       StartAt = "end"
)

func resolveStartAt(value StartAt) (StartAt, error) {
	switch value {
	case "", StartAtBeginning:
		return StartAtBeginning, nil
	case StartAtEnd:
		return StartAtEnd, nil
	default:
		return "", errors.New("invalid file initial start mode")
	}
}
