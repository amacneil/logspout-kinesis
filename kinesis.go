package kinesis

import (
	"errors"
	"os"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/gliderlabs/logspout/router"
	"github.com/remind101/logspout-kinesis/kineprod"
)

func init() {
	router.AdapterFactories.Register(NewKinesisAdapter, "kinesis")
}

var (
	ErrMissingTagKey   = errors.New("the tag key is empty, check your template KINESIS_STREAM_TAG_KEY.")
	ErrMissingTagValue = errors.New("the tag value is empty, check your template KINESIS_STREAM_TAG_VALUE.")
)

type KinesisAdapter struct {
	Streams    map[string]*kineprod.Stream
	StreamTmpl *template.Template
	TagTmpl    *template.Template
	PKeyTmpl   *template.Template
}

func NewKinesisAdapter(route *router.Route) (router.LogAdapter, error) {
	sTmpl, err := compileTmpl("KINESIS_STREAM_TEMPLATE")
	if err != nil {
		return nil, err
	}

	tagTmpl, err := compileTmpl("KINESIS_STREAM_TAG_VALUE")
	if err != nil {
		return nil, err
	}

	pTmpl, err := compileTmpl("KINESIS_PARTITION_KEY_TEMPLATE")
	if err != nil {
		return nil, err
	}

	streams := make(map[string]*kineprod.Stream)

	return &KinesisAdapter{
		Streams:    streams,
		StreamTmpl: sTmpl,
		TagTmpl:    tagTmpl,
		PKeyTmpl:   pTmpl,
	}, nil
}

func (a *KinesisAdapter) Stream(logstream chan *router.Message) {
	for m := range logstream {
		sn, err := executeTmpl(a.StreamTmpl, m)
		if err != nil {
			logErr(err)
			break
		}

		if sn == "" {
			debugLog("the stream name is empty, couldn't match the template. Skipping the log.\n")
			continue
		}

		if s, ok := a.Streams[sn]; ok {
			logErr(s.Write(m))
		} else {
			tags, err := tags(a.TagTmpl, m)
			if err != nil {
				logErr(err)
				break
			}

			s := kineprod.New(sn, tags, a.PKeyTmpl)
			s.Start()
			s.Writer.Start()
			a.Streams[sn] = s
		}
	}
}

func tags(tmpl *template.Template, m *router.Message) (*map[string]*string, error) {
	tagKey := os.Getenv("KINESIS_STREAM_TAG_KEY")
	if tagKey == "" {
		return nil, ErrMissingTagKey
	}

	tagValue, err := executeTmpl(tmpl, m)
	if err != nil {
		return nil, err
	}

	if tagValue == "" {
		return nil, ErrMissingTagValue
	}

	return &map[string]*string{
		tagKey: aws.String(tagValue),
	}, nil
}
