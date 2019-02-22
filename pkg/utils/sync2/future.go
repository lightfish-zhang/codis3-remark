// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package sync2

import "sync"

// golang 技巧， 对 sync 进行封装，添加成员 map 存储返回值
type Future struct {
	sync.Mutex
	wait sync.WaitGroup
	vmap map[string]interface{}
}

func (f *Future) lazyInit() {
	if f.vmap == nil {
		f.vmap = make(map[string]interface{})
	}
}

func (f *Future) Add() {
	f.wait.Add(1)
}

func (f *Future) Done(key string, val interface{}) {
	f.Lock()
	defer f.Unlock()
	f.lazyInit()
	f.vmap[key] = val
	f.wait.Done()
}

func (f *Future) Wait() map[string]interface{} {
	f.wait.Wait()
	f.Lock()
	defer f.Unlock()
	f.lazyInit()
	return f.vmap
}
