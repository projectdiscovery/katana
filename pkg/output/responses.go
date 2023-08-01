package output

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"

	errorutil "github.com/projectdiscovery/utils/errors"
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
	return u.Host, nil
}

func createHostDir(storeResponseFolder, domain string) string {
	_ = os.MkdirAll(filepath.Join(storeResponseFolder, domain), os.ModePerm)
	return filepath.Join(storeResponseFolder, domain)
}

func getResponseFile(storeResponseFolder, URL string) (string, *fileWriter, error) {
	domain, err := getResponseHost(URL)
	if err != nil {
		return "", nil, err
	}
	fileName := getResponseFileName(storeResponseFolder, domain, URL)
	output, err := newFileOutputWriter(fileName)
	if err != nil {
		return "", nil, errorutil.NewWithTag("output", "could not create output file").Wrap(err)
	}

	return fileName, output, nil
}

func getResponseFileName(storeResponseFolder, domain, URL string) string {
	folder := createHostDir(storeResponseFolder, domain)
	file := getResponseHash(URL) + ".txt"
	return filepath.Join(folder, file)
}

func updateIndex(storeResponseFolder string, result *Result) error {
	index, err := os.OpenFile(filepath.Join(storeResponseFolder, indexFile), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer index.Close()

	builder := &bytes.Buffer{}

	domain, err := getResponseHost(result.Request.URL)
	if err != nil {
		return err
	}

	builder.WriteString(getResponseFileName(storeResponseFolder, domain, result.Request.URL))
	builder.WriteRune(' ')
	builder.WriteString(result.Request.URL)
	builder.WriteRune(' ')
	builder.WriteString("(" + result.Response.Resp.Status + ")")
	builder.WriteRune('\n')

	if _, writeErr := index.Write(builder.Bytes()); writeErr != nil {
		return errorutil.NewWithTag("output", "could not update index").Wrap(err)
	}

	return nil
}
