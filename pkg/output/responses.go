package output

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	urlutil "github.com/projectdiscovery/utils/url"
)

func getResponseHash(URL string) string {
	hash := sha1.Sum([]byte(URL))
	return hex.EncodeToString(hash[:])
}

func (w *StandardWriter) formatResponse(resp *http.Response) ([]byte, error) {
	builder := &bytes.Buffer{}

	builder.WriteString(resp.Request.URL.String())
	builder.WriteString("\n\n\n")

	builder.WriteString(resp.Request.Method)
	builder.WriteString(" ")
	path := resp.Request.URL.Path
	if resp.Request.URL.Fragment != "" {
		path = path + "#" + resp.Request.URL.Fragment
	}
	builder.WriteString(path)
	builder.WriteString(" ")
	builder.WriteString(resp.Request.Proto)
	builder.WriteString("\n")
	builder.WriteString("Host: " + resp.Request.Host)
	builder.WriteRune('\n')
	for k, v := range resp.Request.Header {
		builder.WriteString(k + ": " + strings.Join(v, "; ") + "\n")
	}
	builder.WriteString("\n\n")

	builder.WriteString(resp.Proto)
	builder.WriteString(" ")
	builder.WriteString(resp.Status)
	builder.WriteString("\n")
	for k, v := range resp.Header {
		builder.WriteString(k + ": " + strings.Join(v, "; ") + "\n")
	}
	builder.WriteString("\n")
	body, _ := io.ReadAll(resp.Body)
	builder.WriteString(string(body))

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
	_ = os.MkdirAll(filepath.Join(storeResponseFolder, domain), os.ModePerm)
	return filepath.Join(storeResponseFolder, domain)
}

func getResponseFile(storeResponseFolder, URL string) (*fileWriter, error) {
	domain, err := getResponseHost(URL)
	if err != nil {
		return nil, err
	}
	output, err := newFileOutputWriter(getResponseFileName(storeResponseFolder, domain, URL))
	if err != nil {
		return nil, errors.Wrap(err, "could not create output file")
	}

	return output, nil
}

func getResponseFileName(storeResponseFolder, domain, URL string) string {
	folder := createHostDir(storeResponseFolder, domain)
	file := getResponseHash(URL) + ".txt"
	return filepath.Join(folder, file)
}

func updateIndex(storeResponseFolder string, resp *http.Response) error {
	index, err := os.OpenFile(filepath.Join(storeResponseFolder, indexFile), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer index.Close()

	builder := &bytes.Buffer{}

	domain, err := getResponseHost(resp.Request.URL.String())
	if err != nil {
		return err
	}

	builder.WriteString(getResponseFileName(storeResponseFolder, domain, resp.Request.URL.String()))
	builder.WriteRune(' ')
	builder.WriteString(resp.Request.URL.String())
	builder.WriteRune(' ')
	builder.WriteString("(" + resp.Status + ")")
	builder.WriteRune('\n')

	if _, writeErr := index.Write(builder.Bytes()); writeErr != nil {
		return errors.Wrap(err, "could not update index")
	}

	return nil
}
