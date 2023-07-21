package utils

import (
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/require"
)

var htmlFormExample = `<html>
<head>
	<title>HTML Form Test</title>
</head>
<body>
	<form method="POST" action="/test">  
		<input type="text" name="firstname"><br> 
		<textarea name=textarea1></textarea> 
		<select name=select1></select> 
		<input type=text /> 
	</form>  
	<form method=post action=https://abs.example.com></form>
	<form action=//prel.example.com></form>
	<form action=\\unc.example.com></form>
	<form action=/root_rel></form>
	<form action=rel_path></form>
	<form></form>
</body>
</html>`

func TestParseFormFields(t *testing.T) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(htmlFormExample))
	require.NoError(t, err, "could not read document")

	document.Url, _ = url.Parse("https://example.com/path")
	forms := ParseFormFields(document)

	require.Equal(t, "https://example.com/test", forms[0].Action)
	require.Equal(t, "POST", forms[0].Method)
	require.Equal(t, "POST", forms[1].Method)
	require.Equal(t, "https://abs.example.com", forms[1].Action)
	require.Equal(t, "GET", forms[2].Method)
	require.Equal(t, "//prel.example.com", forms[2].Action)
	require.Equal(t, "GET", forms[3].Method)
	require.Equal(t, "\\\\unc.example.com", forms[3].Action)
	require.Equal(t, "GET", forms[4].Method)
	require.Equal(t, "https://example.com/root_rel", forms[4].Action)
	require.Equal(t, "GET", forms[5].Method)
	require.Equal(t, "https://example.com/path/rel_path", forms[5].Action)
	require.Equal(t, "GET", forms[6].Method)
	require.Equal(t, "https://example.com/path", forms[6].Action)
	require.Contains(t, forms[0].Parameters, "firstname")
	require.Contains(t, forms[0].Parameters, "textarea1")
	require.Contains(t, forms[0].Parameters, "select1")
	require.Equal(t, 3, len(forms[0].Parameters), "found more or less parameters than where present")
	require.Equal(t, 7, len(forms), "found more or less forms than where present")
}
