package utils

import (
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
</body>
</html>`

func TestParseFormFields(t *testing.T) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(htmlFormExample))
	require.NoError(t, err, "could not read document")

	forms := ParseFormFields(document)

	require.Equal(t, "/test", forms[0].Action)
	require.Equal(t, "POST", forms[0].Method)
	require.Equal(t, "", forms[0].Enctype)
	require.Contains(t, forms[0].Parameters, "firstname")
	require.Contains(t, forms[0].Parameters, "textarea1")
	require.Contains(t, forms[0].Parameters, "select1")
	require.Equal(t, 1, len(forms), "found more or less params than where present")
}
