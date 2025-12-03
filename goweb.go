package goweb

import (
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/swishcloud/gostudy/logger"
)

type Engine struct {
	RouterGroup
	trees             []methodTree
	ConcurrenceNumSem chan int
	WM                *WidgetManager
	Logger            *log.Logger
}

func Default() *Engine {
	engine := Engine{}
	engine.RouterGroup.engine = &engine
	engine.ConcurrenceNumSem = make(chan int, 5)
	engine.WM = NewWidgetManager()
	engine.Logger = logger.NewLogger(os.Stdout, "GOWEB")
	return &engine
}

type HandlerFunc func(ctx *Context)
type HandlersChain []HandlerFunc

func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(1 * time.Second)
		timeout <- true
	}()
	context := &Context{Engine: engine, Request: req, CT: time.Now(), Signal: make(chan int), Data: make(map[string]interface{}), FuncMap: map[string]interface{}{}}
	context.Writer = w
	context.index = -1
	context.FuncMap["formatTime"] = func(t time.Time, layout string) (string, error) {
		if layout == "" {
			layout = "01/02/2006 15:04"
		}
		tom := 0
		c, err := context.Request.Cookie("tom")
		if err == nil {
			tom, err = strconv.Atoi(c.Value)
			if err != nil {
				panic(err)
			}
		}
		t = t.Add(-time.Duration(int64(time.Minute) * int64(tom)))
		return t.Format(layout), nil
	}

	context.FuncMap["formatTimeString"] = func(t_str string, layout string) (string, error) {
		if layout == "" {
			layout = "01/02/2006 15:04"
		}
		tom := 0
		c, err := context.Request.Cookie("tom")
		if err == nil {
			tom, err = strconv.Atoi(c.Value)
			if err != nil {
				panic(err)
			}
		}
		t, err := time.Parse(time.RFC3339Nano, t_str)
		if err != nil {
			panic(err)
		}
		t = t.Add(-time.Duration(int64(time.Minute) * int64(tom)))
		return t.Format(layout), nil
	}
	context.FuncMap["format_file_size"] = func(sizeStr string) (string, error) {
		size, err := strconv.ParseFloat(sizeStr, 64)
		if err != nil {
			return "", err
		}
		if size > 1024*1024*1024 {
			return strconv.FormatFloat(size/1024/1024/1024, 'f', 2, 64) + " gb", nil
		} else if size > 1024*1024 {
			return strconv.FormatFloat(size/1024/1024, 'f', 2, 64) + " mb", nil
		} else if size > 1024 {
			return strconv.FormatFloat(size/1024, 'f', 2, 64) + " kb", nil
		} else {
			return strconv.FormatFloat(size, 'f', 0, 64) + " bytes", nil
		}
	}

	path := context.Request.URL.Path
	engine.Logger.Println("Incoming request:", path, "Remote IP:", context.Request.RemoteAddr)
	select {
	case engine.ConcurrenceNumSem <- 1:
		var handlers HandlersChain
		for _, v := range engine.trees {
			if v.root.path == path || v.root.regexp != nil && v.root.regexp.MatchString(path) {
				if v.method == context.Request.Method {
					handlers = v.root.handlers
					break
				}
			}
		}
		context.handlers = handlers
		safelyHandle(engine, context)
		<-engine.ConcurrenceNumSem
	case <-timeout:
		engine.Logger.Println(path, "server overload")
		_, err := context.Writer.Write([]byte("server overload"))
		if err != nil {
			engine.Logger.Println(err)
		}
	}
}
func safelyHandle(engine *Engine, c *Context) {
	defer func() {
		if err := recover(); err != nil {
			err_desc := fmt.Sprintf("%s", err)
			_, err := c.Writer.Write([]byte(err_desc))
			if err != nil {
				engine.Logger.Println(err)
			}
		}
	}()
	defer func() {
		if err := recover(); err != nil {
			err_desc := fmt.Sprintf("%s", err)
			c.Err = errors.New(err_desc)
			engine.Logger.Println(err)
		}
		engine.WM.HandlerWidget.Post_Process(c)
	}()
	engine.WM.HandlerWidget.Pre_Process(c)
	if c.handlers == nil {
		c.Err = errors.New("page not found")
		c.Writer.WriteHeader(404)
	} else {
		err := c.Request.ParseForm()
		if err != nil {
			panic(err)
		}
		c.Next()
	}
}

func (ctx *Context) RenderPage(data interface{}, filenames ...string) {
	tmpl := template.New(path.Base(filenames[0])).Funcs(ctx.FuncMap)
	tmpl, err := tmpl.ParseFiles(filenames...)
	if err != nil {
		ctx.Engine.Logger.Println(err)
		ctx.Writer.Write([]byte(fmt.Sprintf("%s", err)))
		return
	}
	err = tmpl.Execute(ctx.Writer, data)
	if err != nil {
		ctx.Engine.Logger.Println(err)
		return
	}
}
