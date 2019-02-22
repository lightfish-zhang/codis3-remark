// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

const MaxGroupId = 9999

//每个Group除了id，还有一个属性就是GroupServer。每个GroupServer有自己的地址、数据中心、action等等
type Group struct {
	Id      int            `json:"id"`
	Servers []*GroupServer `json:"servers"`

	Promoting struct {
		Index int    `json:"index,omitempty"`
		State string `json:"state,omitempty"`
	} `json:"promoting"`

	OutOfSync bool `json:"out_of_sync"`
}

type GroupServer struct {
	Addr       string `json:"server"`
	DataCenter string `json:"datacenter"`

	Action struct {
		Index int    `json:"index,omitempty"`
		State string `json:"state,omitempty"`
	} `json:"action"`

	ReplicaGroup bool `json:"replica_group"`
}

func (g *Group) Encode() []byte {
	return jsonEncode(g)
}
