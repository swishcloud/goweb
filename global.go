package goweb

import "github.com/microcosm-cc/bluemonday"

func init() {
	bluemondayPolicy = bluemonday.UGCPolicy()
}

var bluemondayPolicy *bluemonday.Policy
