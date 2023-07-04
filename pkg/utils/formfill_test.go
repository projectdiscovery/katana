package utils

import (
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/require"
)

var htmlFormInputExample = `<html>
<head>
	<title>HTML Form Test</title>
</head>
<body>
	<form action="/">  
		<label>User id: </label>  
		<input type="text" name="firstname"><br>  
		<label>Password: </label>  
		<input type="Password" name="password"><br> 
		<p>Kindly Select your favorite color</p>  
		<input type="radio" name="color" value="red"> Red <br>  
		<input type="radio" name="color" value="blue"> blue <br>  
		<input type="radio" name="color" value="green">green <br>   
		<p>Kindly Select your favourite sports</p>  
		<input type="checkbox" name="sport1" value="cricket">Cricket<br>  
		<input type="checkbox" name="sport2" value="tennis">Tennis<br>  
		<input type="checkbox" name="sport3" value="football">Football<br>  
		<input type="color" name="upclick" value="#a52a2a"> Upclick<br><br>  
		<input type="date" name="Startdate"> Start date:<br><br>  
		<label>Enter your age: </label>  
		<input type="number" name="num" min="50" max="80">  
		<label><b>Enter your Telephone Number(in format of xxx-xxx-xxxx):</b></label>  
		<input type="tel" name="telephone" pattern="[0-9]{3}-[0-9]{3}-[0-9]{4}" required>  
		<br><br><input type="submit" value="submit">   
	</form>  
</body>
</html>`

func TestFormInputFillSuggestions(t *testing.T) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(htmlFormInputExample))
	require.NoError(t, err, "could not read document")

	document.Find("form[action]").Each(func(i int, item *goquery.Selection) {
		queryValuesWriter := make(url.Values)
		formInputs := []FormInput{}

		item.Find("input").Each(func(index int, item *goquery.Selection) {
			if len(item.Nodes) == 0 {
				return
			}
			formInputs = append(formInputs, ConvertGoquerySelectionToFormInput(item))
		})

		dataMap := FormInputFillSuggestions(formInputs)
		dataMap.Iterate(func(key, value string) bool {
			if key == "" || value == "" {
				return true
			}
			queryValuesWriter.Set(key, value)
			return true
		})
		value := queryValuesWriter.Encode()
		require.Equal(t, "Startdate=katana&color=red&firstname=katana&num=51&password=katana&sport1=cricket&sport2=tennis&sport3=football&telephone=katanaP%40assw0rd1&upclick=%23a52a2a", value, "could not get correct encoded form")
	})
}
