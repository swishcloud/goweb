package goweb

import "github.com/microcosm-cc/bluemonday"

func init() {
	bluemondayPolicy = bluemonday.NewPolicy()
	bluemondayPolicy.AllowStandardURLs()
	bluemondayPolicy.AllowAttrs("href").OnElements("a", "area")
	bluemondayPolicy.AllowAttrs("src").OnElements("img")
	bluemondayPolicy.AllowAttrs("class").OnElements("code", "span")
	bluemondayPolicy.AllowElements("h1", "h2", "h3", "h4", "h5", "h6")
	bluemondayPolicy.AllowElements("p", "ol", "li", "br", "pre", "code", "span", "del")
}

var bluemondayPolicy *bluemonday.Policy
