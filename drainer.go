package kinesis

import (
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/kinesis"
)

type Drainer struct {
	Buffer *recordBuffer
}

func newDrainer(a *KinesisAdapter, streamName string) {
	buffer, err := newRecordBuffer(a.Client, streamName)
	if err != nil {
		logErr(err)
	}

	d := &Drainer{Buffer: buffer}
	go d.Drain()

	err = createStream(a, streamName)
	if err != nil {
		if reqErr, ok := err.(awserr.RequestFailure); ok {
			if reqErr.Code() == "ResourceInUseException" {
				a.addDrainer(streamName, d)
			} else {
				logErr(err)
			}
		} else {
			logErr(err)
		}
	} else {
		go waitForActive(a, d)
	}
}

// Drain flushes the buffer every second.
func (d *Drainer) Drain() {
	for _ = range time.Tick(time.Second * 1) {
		logErr(d.Buffer.Flush())
	}
}

func createStream(a *KinesisAdapter, streamName string) error {
	_, err := a.Client.CreateStream(&kinesis.CreateStreamInput{
		ShardCount: aws.Int64(1),
		StreamName: aws.String(streamName),
	})

	return err
}

func waitForActive(a *KinesisAdapter, d *Drainer) {
	streamName := *d.Buffer.input.StreamName
	var streamStatus string

	params := &kinesis.DescribeStreamInput{StreamName: aws.String(streamName)}
	resp := &kinesis.DescribeStreamOutput{}
	for {
		resp, _ = a.Client.DescribeStream(params)
		if streamStatus = *resp.StreamDescription.StreamStatus; streamStatus == "ACTIVE" {
			a.addDrainer(streamName, d)
			break
		} else {
			time.Sleep(4 * time.Second)
		}

		log.Printf("kinesis: STREAM '%s' STATUS: %s\n", streamName, streamStatus)
	}
}
