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
	<form method=post></form>
	<form action="/test2">
</body>
</html>`

func TestParseFormFields(t *testing.T) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(htmlFormExample))
	require.NoError(t, err, "could not read document")

	forms := ParseFormFields(document)

	require.Equal(t, "/test", forms[0].Action)
	require.Equal(t, "POST", forms[0].Method)
	require.Equal(t, "POST", forms[1].Method)
	require.Equal(t, "/test2", forms[2].Action)
	require.Equal(t, "", forms[0].Enctype)
	require.Contains(t, forms[0].Parameters, "firstname")
	require.Contains(t, forms[0].Parameters, "textarea1")
	require.Contains(t, forms[0].Parameters, "select1")
	require.Equal(t, 3, len(forms[0].Parameters), "found more or less parameters than where present")
	require.Equal(t, 3, len(forms), "found more or less forms than where present")
}
