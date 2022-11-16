package output

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"os"

	"github.com/pkg/errors"
	urlutil "github.com/projectdiscovery/utils/url"
)

func getResponseHash(URL string) string {
	hash := sha1.Sum([]byte(URL))
	return hex.EncodeToString(hash[:])
}

func (w *StandardWriter) formatResponse(output *Result) ([]byte, error) {
	builder := &bytes.Buffer{}

	builder.WriteString(output.URL)
	builder.WriteRune('\n')

	if output.Timestamp.String() != "" {
		builder.WriteString(output.Timestamp.String())
		builder.WriteRune('\n')
	}

	if output.Method != "" {
		builder.WriteString(output.Method)
		builder.WriteRune('\n')
	}

	if output.Body != "" {
		builder.WriteString(output.Body)
		builder.WriteRune('\n')
	}

	if output.Source != "" {
		builder.WriteString(output.Source)
		builder.WriteRune('\n')
	}

	if output.Tag != "" {
		builder.WriteString(output.Tag)
		builder.WriteRune('\n')
	}

	if output.Attribute != "" {
		builder.WriteString(output.Attribute)
		builder.WriteRune('\n')
	}

	return builder.Bytes(), nil
}

func getResponseHost(URL string) (string, error) {
	u, err := urlutil.ParseWithScheme(URL)
	if err != nil {
		return "", err
	}

	return u.Host, nil
}

func createHostDir(storeResponseFolder, domain string) string {
	_ = os.MkdirAll(storeResponseFolder+"/"+domain, os.ModePerm)
	return storeResponseFolder + "/" + domain
}

func getResponseFile(storeResponseFolder, URL string) (*fileWriter, error) {
	domain, err := getResponseHost(URL)
	if err != nil {
		return nil, err
	}
	folder := createHostDir(storeResponseFolder, domain)
	file := getResponseHash(URL)
	output, err := newFileOutputWriter(folder + "/" + file)
	if err != nil {
		return nil, errors.Wrap(err, "could not create output file")
	}

	return output, nil
}
