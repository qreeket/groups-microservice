package utils

import "go.mongodb.org/mongo-driver/bson"

// StreamDoc is a struct that represents a stream document (FullDocument & OperationType)
type StreamDoc struct {
	FullDocument  bson.M `bson:"fullDocument"`
	OperationType string `bson:"operationType"`
}

func (ce *StreamDoc) GetFullDocument(result interface{}) error {
	bytes, err := bson.Marshal(ce.FullDocument)
	if err != nil {
		return err
	}
	if err := bson.Unmarshal(bytes, result); err != nil {
		return err
	}
	return nil
}
