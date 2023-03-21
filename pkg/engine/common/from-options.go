package common

import (
	"context"
	"time"

	"github.com/bxcodec/faker/v4/pkg/options"
	"github.com/projectdiscovery/katana/pkg/engine/parser/files"
	"github.com/projectdiscovery/katana/pkg/types"
	errorutil "github.com/projectdiscovery/utils/errors"
)

func KnownFilesFromOptions(crawlerOptions *types.CrawlerOptions) (*files.KnownFiles, error) {
	if crawlerOptions.Options.KnownFiles != "" {
		httpclient, _, err := BuildHttpClient(crawlerOptions.Dialer, crawlerOptions.Options, nil)
		if err != nil {
			return nil, errorutil.New("could not create http client").Wrap(err)
		}
		return files.New(httpclient, options.Options.KnownFiles), nil
	}

	return nil, nil
}

func ContextFromOptions(crawlerOptions *types.CrawlerOptions) (context.Context, context.CancelFunc) {
	if crawlerOptions.Options.CrawlDuration > 0 {
		return context.WithTimeout(context.Background(), time.Duration(crawlerOptions.Options.CrawlDuration)*time.Second)
	}
	return context.WithCancel(context.Background())
}
