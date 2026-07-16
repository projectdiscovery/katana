package utils

import (
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	mapsutil "github.com/projectdiscovery/utils/maps"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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
		<p>Kindly Select your favourite food</p>  
		<select name="food" id="food">
			<option value="pizza">Pizza</option>
			<option value="burger">Burger</option>
			<option value="pasta" selected>Pasta</option>
		</select>
		<p>Kindly Select your favourite country</p>  
		<select name="country" id="country">
			<option value="india">India</option>
			<option value="usa">USA</option>
			<option value="uk">UK</option>
			<option value="canada">Canada</option>
		</select>
		<label><b>Write some words about yourself:</b></label>
		<textarea id="message" name="message" rows="10" cols="50">
			Write something here
		</textarea>
		
		
		<br><br><input type="submit" value="submit">
		
	</form>  
</body>
</html>`

func TestFormInputFillSuggestions(t *testing.T) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(htmlFormInputExample))
	require.NoError(t, err, "could not read document")

	document.Find("form[action]").Each(func(i int, item *goquery.Selection) {
		queryValuesWriter := make(url.Values)
		formFields := []interface{}{}

		item.Find("input, textarea, select").Each(func(index int, item *goquery.Selection) {
			if len(item.Nodes) == 0 {
				return
			}
			formFields = append(formFields, ConvertGoquerySelectionToFormField(item))
		})

		dataMap := FormFillSuggestions(formFields)
		dataMap.Iterate(func(key, value string) bool {
			if key == "" || value == "" {
				return true
			}
			queryValuesWriter.Set(key, value)
			return true
		})
		value := queryValuesWriter.Encode()
		require.Equal(t, "Startdate=katana&color=green&country=india&firstname=katana&food=pasta&message=katana&num=51&password=katana&sport1=cricket&sport2=tennis&sport3=football&telephone=katanaP%40assw0rd1&upclick=%23a52a2a", value, "could not get correct encoded form")
	})
}

func TestDefaultFormFillDataIsStatic(t *testing.T) {
	require.Contains(t, DefaultFormFillData.Email, "@example.org")
	require.Equal(t, "#e66465", DefaultFormFillData.Color)
	require.Equal(t, "katanaP@assw0rd1", DefaultFormFillData.Password)
	require.Equal(t, "2124567890", DefaultFormFillData.PhoneNumber)
	require.Equal(t, "katana", DefaultFormFillData.Placeholder)
}

func TestDefaultFormDataInitMatchesDefault(t *testing.T) {
	require.Equal(t, DefaultFormFillData.Color, FormData.Color)
	require.Equal(t, DefaultFormFillData.Password, FormData.Password)
	require.Equal(t, DefaultFormFillData.PhoneNumber, FormData.PhoneNumber)
	require.Equal(t, DefaultFormFillData.Placeholder, FormData.Placeholder)
}

func TestResolveDoesNotAlterDefaultValues(t *testing.T) {
	data := DefaultFormFillData
	data.Resolve()

	require.Contains(t, data.Email, "@example.org", "default email has no parens, should stay as-is")
	require.Equal(t, "#e66465", data.Color)
	require.Equal(t, "katanaP@assw0rd1", data.Password)
	require.Equal(t, "2124567890", data.PhoneNumber)
	require.Equal(t, "katana", data.Placeholder)
}

func TestFormInputFillSuggestionsWithDefaults(t *testing.T) {
	original := FormData
	defer func() { FormData = original }()

	FormData = DefaultFormFillData

	inputs := []FormInput{
		{Name: "user_email", Type: "email"},
		{Name: "user_pass", Type: "password"},
		{Name: "user_phone", Type: "tel"},
		{Name: "nickname", Type: "text"},
		{Name: "bg", Type: "color"},
	}
	suggestions := FormInputFillSuggestions(inputs)

	emailVal, _ := suggestions.Get("user_email")
	require.Equal(t, DefaultFormFillData.Email, emailVal)

	passVal, _ := suggestions.Get("user_pass")
	require.Equal(t, DefaultFormFillData.Password, passVal)

	phoneVal, _ := suggestions.Get("user_phone")
	require.Equal(t, DefaultFormFillData.Password, phoneVal, "tel type uses Password field")

	nickVal, _ := suggestions.Get("nickname")
	require.Equal(t, DefaultFormFillData.Placeholder, nickVal)

	colorVal, _ := suggestions.Get("bg")
	require.Equal(t, DefaultFormFillData.Color, colorVal)
}

func TestFormSelectFillPicksSelectedOption(t *testing.T) {
	selects := []FormSelect{
		{
			Name: "lang",
			SelectOptions: []SelectOption{
				{Value: "en"},
				{Value: "fr", Selected: "selected"},
				{Value: "de"},
			},
		},
	}
	result := FormSelectFill(selects)
	val, _ := result.Get("lang")
	require.Equal(t, "fr", val)
}

func TestFormSelectFillDefaultsToFirst(t *testing.T) {
	selects := []FormSelect{
		{
			Name: "country",
			SelectOptions: []SelectOption{
				{Value: "us"},
				{Value: "uk"},
			},
		},
	}
	result := FormSelectFill(selects)
	val, _ := result.Get("country")
	require.Equal(t, "us", val)
}

func TestFormTextAreaFillUsesPlaceholder(t *testing.T) {
	original := FormData
	defer func() { FormData = original }()
	FormData = DefaultFormFillData

	areas := []FormTextArea{
		{Name: "bio"},
		{Name: "notes"},
	}
	result := FormTextAreaFill(areas)

	bioVal, _ := result.Get("bio")
	require.Equal(t, "katana", bioVal)
	notesVal, _ := result.Get("notes")
	require.Equal(t, "katana", notesVal)
}

func TestFormInputRadioPicksFirstValue(t *testing.T) {
	inputs := []FormInput{
		{Name: "size", Type: "radio", Value: "small"},
		{Name: "size", Type: "radio", Value: "medium"},
		{Name: "size", Type: "radio", Value: "large"},
	}
	result := FormInputFillSuggestions(inputs)
	val, _ := result.Get("size")
	require.Equal(t, "small", val, "radio should keep the first value encountered")
}

func TestFormInputCheckboxKeepsAllValues(t *testing.T) {
	inputs := []FormInput{
		{Name: "opt_a", Type: "checkbox", Value: "a"},
		{Name: "opt_b", Type: "checkbox", Value: "b"},
	}
	result := FormInputFillSuggestions(inputs)
	a, _ := result.Get("opt_a")
	b, _ := result.Get("opt_b")
	require.Equal(t, "a", a)
	require.Equal(t, "b", b)
}

func TestFormInputNumberRespectsMinMaxStep(t *testing.T) {
	attrs := mapsutil.NewOrderedMap[string, string]()
	attrs.Set("min", "10")
	attrs.Set("max", "20")
	attrs.Set("step", "5")
	inputs := []FormInput{
		{Name: "qty", Type: "number", Attributes: attrs},
	}
	result := FormInputFillSuggestions(inputs)
	val, _ := result.Get("qty")
	require.Equal(t, "15", val, "should be min+step = 10+5")
}

func TestFormInputPreservesExistingValue(t *testing.T) {
	inputs := []FormInput{
		{Name: "token", Type: "hidden", Value: "abc123"},
	}
	result := FormInputFillSuggestions(inputs)
	val, _ := result.Get("token")
	require.Equal(t, "abc123", val, "pre-filled values should be kept")
}

func TestCustomFormConfigOverridesDefaults(t *testing.T) {
	original := FormData
	defer func() { FormData = original }()

	yamlConfig := `
email: "custom@test.org"
color: "#112233"
password: "customP@ss"
phone: "9876543210"
placeholder: "custom"
`
	var data FormFillData
	err := yaml.Unmarshal([]byte(yamlConfig), &data)
	require.NoError(t, err)

	data.Resolve()
	FormData = data

	inputs := []FormInput{
		{Name: "em", Type: "email"},
		{Name: "pw", Type: "password"},
		{Name: "ph", Type: "tel"},
		{Name: "nm", Type: "text"},
		{Name: "cl", Type: "color"},
	}
	suggestions := FormInputFillSuggestions(inputs)

	emVal, _ := suggestions.Get("em")
	require.Equal(t, "custom@test.org", emVal)
	pwVal, _ := suggestions.Get("pw")
	require.Equal(t, "customP@ss", pwVal)
	phVal, _ := suggestions.Get("ph")
	require.Equal(t, "customP@ss", phVal, "tel uses Password field")
	nmVal, _ := suggestions.Get("nm")
	require.Equal(t, "custom", nmVal)
	clVal, _ := suggestions.Get("cl")
	require.Equal(t, "#112233", clVal)
}

func TestResolveFieldPlainStrings(t *testing.T) {
	data := FormFillData{
		Email:       "alice@example.com",
		Color:       "#ff0000",
		Password:    "s3cret!",
		PhoneNumber: "5551234567",
		Placeholder: "hello",
	}
	data.Resolve()

	require.Equal(t, "alice@example.com", data.Email)
	require.Equal(t, "#ff0000", data.Color)
	require.Equal(t, "s3cret!", data.Password)
	require.Equal(t, "5551234567", data.PhoneNumber)
	require.Equal(t, "hello", data.Placeholder)
}

func TestResolveFieldDSLExpressions(t *testing.T) {
	data := FormFillData{
		Email:       "rand_email()",
		Password:    `rand_base(16, "")`,
		PhoneNumber: "rand_phone()",
		Placeholder: "rand_first_name()",
		Color:       "#e66465",
	}
	data.Resolve()

	require.Contains(t, data.Email, "@", "rand_email() should produce an email address")
	require.NotEqual(t, "rand_email()", data.Email, "expression should have been evaluated")
	require.Len(t, data.Password, 16)
	require.NotEmpty(t, data.PhoneNumber)
	require.NotEqual(t, "rand_phone()", data.PhoneNumber)
	require.NotEmpty(t, data.Placeholder)
	require.NotEqual(t, "rand_first_name()", data.Placeholder)
	require.Equal(t, "#e66465", data.Color, "plain color value should be unchanged")
}

func TestResolveFieldMixed(t *testing.T) {
	data := FormFillData{
		Email:       "rand_email()",
		Color:       "#abcdef",
		Password:    "myStaticPass!",
		PhoneNumber: "rand_phone()",
		Placeholder: "katana",
	}
	data.Resolve()

	require.Contains(t, data.Email, "@")
	require.NotEqual(t, "rand_email()", data.Email)
	require.Equal(t, "#abcdef", data.Color)
	require.Equal(t, "myStaticPass!", data.Password)
	require.NotEqual(t, "rand_phone()", data.PhoneNumber)
	require.Equal(t, "katana", data.Placeholder)
}

func TestResolveFieldEmpty(t *testing.T) {
	data := FormFillData{}
	data.Resolve()

	require.Empty(t, data.Email)
	require.Empty(t, data.Color)
	require.Empty(t, data.Password)
	require.Empty(t, data.PhoneNumber)
	require.Empty(t, data.Placeholder)
}

func TestResolveFieldRandBase(t *testing.T) {
	data := FormFillData{
		Placeholder: `rand_base(16, "")`,
	}
	data.Resolve()

	require.Len(t, data.Placeholder, 16, "rand_base(16) should produce a 16-char string")
}

func TestResolveProducesUniqueValues(t *testing.T) {
	a := FormFillData{Email: "rand_email()"}
	b := FormFillData{Email: "rand_email()"}
	a.Resolve()
	b.Resolve()

	require.NotEqual(t, a.Email, b.Email, "successive calls should produce different random values")
}

func TestResolveFromYAML(t *testing.T) {
	yamlConfig := `
email: "rand_email()"
color: "#e66465"
password: 'rand_base(16, "")'
phone: "rand_phone()"
placeholder: "rand_first_name()"
`
	var data FormFillData
	err := yaml.Unmarshal([]byte(yamlConfig), &data)
	require.NoError(t, err)

	data.Resolve()

	require.Contains(t, data.Email, "@")
	require.NotEqual(t, "rand_email()", data.Email)
	require.Equal(t, "#e66465", data.Color)
	require.Len(t, data.Password, 16)
	require.NotEmpty(t, data.PhoneNumber)
	require.NotEqual(t, "rand_phone()", data.PhoneNumber)
	require.NotEmpty(t, data.Placeholder)
	require.NotEqual(t, "rand_first_name()", data.Placeholder)
}

func TestResolveFromYAMLPlainValues(t *testing.T) {
	yamlConfig := `
email: "admin@corp.com"
color: "#000000"
password: "hunter2"
phone: "18005551234"
placeholder: "test"
`
	var data FormFillData
	err := yaml.Unmarshal([]byte(yamlConfig), &data)
	require.NoError(t, err)

	data.Resolve()

	require.Equal(t, "admin@corp.com", data.Email)
	require.Equal(t, "#000000", data.Color)
	require.Equal(t, "hunter2", data.Password)
	require.Equal(t, "18005551234", data.PhoneNumber)
	require.Equal(t, "test", data.Placeholder)
}

func TestFormFillSuggestionsWithDSLConfig(t *testing.T) {
	original := FormData
	defer func() { FormData = original }()

	FormData = FormFillData{
		Email:       "rand_email()",
		Password:    `rand_base(16, "")`,
		PhoneNumber: "rand_phone()",
		Placeholder: "rand_first_name()",
		Color:       "#e66465",
	}
	FormData.Resolve()

	inputs := []FormInput{
		{Name: "email", Type: "email"},
		{Name: "pass", Type: "password"},
		{Name: "phone", Type: "tel"},
		{Name: "username", Type: "text"},
	}
	suggestions := FormInputFillSuggestions(inputs)

	emailVal, _ := suggestions.Get("email")
	require.Contains(t, emailVal, "@", "email field should have a resolved email")

	passVal, _ := suggestions.Get("pass")
	require.NotEmpty(t, passVal, "password field should be filled")

	phoneVal, _ := suggestions.Get("phone")
	require.NotEmpty(t, phoneVal, "phone field should be filled")

	usernameVal, _ := suggestions.Get("username")
	require.NotEmpty(t, usernameVal, "text field should use resolved placeholder")
}

func TestMixedConfigEndToEnd(t *testing.T) {
	original := FormData
	defer func() { FormData = original }()

	yamlConfig := `
email: "rand_email()"
color: "#facade"
password: "H@rdC0ded!"
phone: "rand_phone()"
placeholder: "static-user"
`
	var data FormFillData
	err := yaml.Unmarshal([]byte(yamlConfig), &data)
	require.NoError(t, err)
	data.Resolve()
	FormData = data

	require.Contains(t, FormData.Email, "@")
	require.NotEqual(t, "rand_email()", FormData.Email)
	require.Equal(t, "#facade", FormData.Color)
	require.Equal(t, "H@rdC0ded!", FormData.Password)
	require.NotEqual(t, "rand_phone()", FormData.PhoneNumber)
	require.Equal(t, "static-user", FormData.Placeholder)

	htmlForm := `<html><body>
	<form action="/submit">
		<input type="email" name="contact_email">
		<input type="password" name="secret">
		<input type="tel" name="mobile">
		<input type="text" name="display_name">
		<input type="color" name="theme">
		<textarea name="bio"></textarea>
		<select name="role">
			<option value="admin">Admin</option>
			<option value="user" selected>User</option>
		</select>
		<input type="submit" value="Go">
	</form>
	</body></html>`

	document, err := goquery.NewDocumentFromReader(strings.NewReader(htmlForm))
	require.NoError(t, err)

	document.Find("form[action]").Each(func(i int, item *goquery.Selection) {
		var formFields []interface{}
		item.Find("input, textarea, select").Each(func(_ int, el *goquery.Selection) {
			if len(el.Nodes) == 0 {
				return
			}
			field := ConvertGoquerySelectionToFormField(el)
			if field != nil {
				formFields = append(formFields, field)
			}
		})

		suggestions := FormFillSuggestions(formFields)
		values := make(url.Values)
		suggestions.Iterate(func(k, v string) bool {
			if k != "" && v != "" {
				values.Set(k, v)
			}
			return true
		})

		require.Contains(t, values.Get("contact_email"), "@", "email should be a resolved random address")
		require.Equal(t, "H@rdC0ded!", values.Get("secret"), "password should be the static value")
		require.Equal(t, "H@rdC0ded!", values.Get("mobile"), "tel uses Password field value")
		require.Equal(t, "static-user", values.Get("display_name"), "text should use static placeholder")
		require.Equal(t, "#facade", values.Get("theme"), "color should be the static value")
		require.Equal(t, "static-user", values.Get("bio"), "textarea should use static placeholder")
		require.Equal(t, "user", values.Get("role"), "select should pick the selected option")
	})
}
