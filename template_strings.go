package doorman

import "html/template"

const signinRequestHTML = `
<html>
<head>
<meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<style>
* {
  font-family: sans-serif;
  text-align: center;
}
h3 {
	margin-top: 50px;
}
.answer {
  margin-top: 50px;
}
</style>
<body>
		<h3>Signin Request</h3>
		<div>A signin request from user <b>%s</b> originated from IP <b>%s</b></div>
		<div class="answer">Click <a href="allow?t=%s&a=yes">YES</a> to allow this request.
</body>
</html>
`
const noSigninRequestHTML = `
<html>
<head>
<meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<style>
* {
  font-family: sans-serif;
  text-align: center;
}
h3 {
	margin-top: 50px;
}
.answer {
  margin-top: 50px;
}
</style>
<body>
		<h3>Signin Request</h3>
		<div class="answer">No active request found</div>
</body>
</html>
`

const signinAcceptedHTML = `
<html>
<head>
<meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<style>
* {
  font-family: sans-serif;
  text-align: center;
}
h3 {
	margin-top: 50px;
}
</style>
<body>
		<h3>Signin Request accepted</h3>
</body>
</html>
`

const emailLinkNotificationTmpl = `
<html>
<head>
<style>
* {
  font-family: sans-serif;
}
.stack {
  max-width: 400px;
  margin: auto;
}
.root {
  max-width: 400px;
  min-width: 400px;
  border-radius: 10px;
  padding: 5px;
  z-index: 10px;
}
.header {
  padding-top:10px;
  padding-left:120px;
  height: 55px;
  background-color: #0c609c;
  color: white;
  font-size: 1.5em;
  border-radius: 5px;
  margin-bottom: 10px;
}
.footer {
  margin-top: 20px;
  font-size: 0.8em;
}
.content {

}

</style>
</head>
<body>
	<div class="stack">
    	<div class="root">
		<div class="header"><div>Login request</div></div>
		<div class="content">
				A login request was triggered. Click <a href="{{ .Loginlink }}">to allow</a>. After {{ .Timeout }} the request will be automatically denied
			</div>
            <div class="footer">
				<div>This is an automatic email sent by a login request. Please ignore this mail if you did not trigger the login.</div>

				{{ if ne .Imprint ""}}
				<a href="{{ .Imprint }}">Imprint</a> &nbsp;
				{{ end }}
				{{ if ne .PrivacyPolicy ""}}
				<a href="{{ .PrivacyPolicy }}">Privacy policy</a>
				{{ end }}

            	<div>Sent: {{ .Sent }}</div>
            </div>
      	</div>
	</div>
</body>
</html>
`

var (
	emailLinkNotification = template.Must(template.New("emaillink").Parse(emailLinkNotificationTmpl))
)
