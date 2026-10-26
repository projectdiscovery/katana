package output

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/utils/errkit"
	urlutil "github.com/projectdiscovery/utils/url"
)

func getResponseHash(URL string) string {
	hash := sha1.Sum([]byte(URL))
	return hex.EncodeToString(hash[:])
}

func (w *StandardWriter) formatResult(result *Result) ([]byte, error) {
	builder := &bytes.Buffer{}

	builder.WriteString(result.Request.URL)
	builder.WriteString("\n\n\n")

	builder.WriteString(result.Request.Raw)

	builder.WriteString("\n\n")

	builder.WriteString(result.Response.Raw)

	return builder.Bytes(), nil
}

func getResponseHost(URL string) (string, error) {
	u, err := urlutil.Parse(URL)
	if err != nil {
		return "", err
	}
	return filepath.Clean(strings.ReplaceAll(u.Host, ":", "_")), nil
}

func createHostDir(storeResponseFolder, domain string) string {
	_ = os.MkdirAll(filepath.Join(storeResponseFolder, domain), os.ModePerm)
	return filepath.Join(storeResponseFolder, domain)
}

func getResponseFile(storeResponseFolder, URL, agent string) (string, *fileWriter, error) {
	domain, err := getResponseHost(URL)
	if err != nil {
		return "", nil, err
	}
	fileName := getResponseFileName(storeResponseFolder, domain, URL, agent)
	output, err := newFileOutputWriter(fileName)
	if err != nil {
		return "", nil, errkit.Wrap(err, "output: could not create output file")
	}

	return fileName, output, nil
}

// getResponseFileName returns the file that stores the response to URL. With
// a crawl as several identities, agent names the identity, so that two
// identities that get the same URL store their responses in two files.
func getResponseFileName(storeResponseFolder, domain, URL, agent string) string {
	folder := createHostDir(storeResponseFolder, domain)
	key := URL
	if agent != "" {
		key = agent + "\x00" + URL
	}
	file := getResponseHash(key) + ".txt"
	return filepath.Join(folder, file)
}

func updateIndex(storeResponseFolder string, result *Result) error {
	index, err := os.OpenFile(filepath.Join(storeResponseFolder, indexFile), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer func() {
		if err := index.Close(); err != nil {
			gologger.Error().Msgf("Error closing index: %v\n", err)
		}
	}()

	builder := &bytes.Buffer{}

	domain, err := getResponseHost(result.Request.URL)
	if err != nil {
		return err
	}

	builder.WriteString(getResponseFileName(storeResponseFolder, domain, result.Request.URL, resultAgent(result)))
	builder.WriteRune(' ')
	builder.WriteString(result.Request.URL)
	builder.WriteRune(' ')
	builder.WriteString("(" + result.Response.Resp.Status + ")")
	builder.WriteRune('\n')

	if _, writeErr := index.Write(builder.Bytes()); writeErr != nil {
		return errkit.Wrap(err, "output: could not update index")
	}

	return nil
}

// resultAgent returns the identity that found result, or "" for a crawl as
// one identity.
func resultAgent(result *Result) string {
	if result == nil || result.Request == nil {
		return ""
	}
	return result.Request.Agent
}
