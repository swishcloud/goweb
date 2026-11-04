package goweb

import "regexp"

type RouterGroup struct {
	engine   *Engine
	Handlers HandlersChain
}

func (group *RouterGroup) Group() *RouterGroup {
	return &RouterGroup{
		Handlers: append(HandlersChain(nil), group.Handlers...),
		engine:   group.engine,
	}
}

func (group *RouterGroup) GET(path string, handler HandlerFunc) {
	group.engine.trees = append(group.engine.trees, methodTree{"GET", &node{path: path, handlers: append(append(HandlersChain(nil), group.Handlers...), handler)}})
}

func (group *RouterGroup) POST(path string, handler HandlerFunc) {
	group.engine.trees = append(group.engine.trees, methodTree{"POST", &node{path: path, handlers: append(append(HandlersChain(nil), group.Handlers...), handler)}})
}

func (group *RouterGroup) PUT(path string, handler HandlerFunc) {
	group.engine.trees = append(group.engine.trees, methodTree{"PUT", &node{path: path, handlers: append(append(HandlersChain(nil), group.Handlers...), handler)}})
}

func (group *RouterGroup) DELETE(path string, handler HandlerFunc) {
	group.engine.trees = append(group.engine.trees, methodTree{"DELETE", &node{path: path, handlers: append(append(HandlersChain(nil), group.Handlers...), handler)}})
}

func (group *RouterGroup) RegexMatch(regexp *regexp.Regexp, handler HandlerFunc) {
	group.engine.trees = append(group.engine.trees, methodTree{"GET", &node{regexp: regexp, handlers: append(append(HandlersChain(nil), group.Handlers...), handler)}})
}

func (group *RouterGroup) Use(middleware ...HandlerFunc) {
	group.Handlers = append(group.Handlers, middleware...)
}
