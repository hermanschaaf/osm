package osm

import (
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/paulmach/go.osm/internal/osmpb"
)

const locMultiple = 10000000.0

var memberTypeMap = map[Type]osmpb.Relation_MemberType{
	TypeNode:     osmpb.Relation_NODE,
	TypeWay:      osmpb.Relation_WAY,
	TypeRelation: osmpb.Relation_RELATION,
}

var memberTypeMapRev = map[osmpb.Relation_MemberType]Type{
	osmpb.Relation_NODE:     TypeNode,
	osmpb.Relation_WAY:      TypeWay,
	osmpb.Relation_RELATION: TypeRelation,
}

func marshalNode(node *Node, ss *stringSet, includeChangeset bool) *osmpb.Node {
	keys, vals := node.Tags.keyValues(ss)
	encoded := &osmpb.Node{
		Id:   int64(node.ID),
		Keys: keys,
		Vals: vals,
		Info: &osmpb.Info{
			Version:   int32(node.Version),
			Timestamp: timeToUnix(node.Timestamp),
			Visible:   proto.Bool(node.Visible),
		},
		Lat: geoToInt64(node.Lat),
		Lon: geoToInt64(node.Lon),
	}

	if node.Committed != nil {
		encoded.Info.Committed = timeToUnixPointer(*node.Committed)
	}

	if includeChangeset {
		encoded.Info.ChangesetId = int64(node.ChangesetID)
		encoded.Info.UserId = int32(node.UserID)
		encoded.Info.UserSid = ss.Add(node.User)
	}

	return encoded
}

func unmarshalNode(encoded *osmpb.Node, ss []string, cs *Changeset) (*Node, error) {
	tags, err := tagsFromStrings(ss, encoded.GetKeys(), encoded.GetVals())
	if err != nil {
		return nil, err
	}

	info := encoded.GetInfo()
	n := &Node{
		ID:          NodeID(encoded.GetId()),
		User:        ss[info.GetUserSid()],
		UserID:      UserID(info.GetUserId()),
		Visible:     info.GetVisible(),
		Version:     int(info.GetVersion()),
		ChangesetID: ChangesetID(info.GetChangesetId()),
		Timestamp:   unixToTime(info.GetTimestamp()),
		Tags:        tags,
		Lat:         float64(encoded.GetLat()) / locMultiple,
		Lon:         float64(encoded.GetLon()) / locMultiple,

		Committed: unixToTimePointer(info.GetCommitted()),
	}

	if cs != nil {
		n.ChangesetID = cs.ID
		n.UserID = cs.UserID
		n.User = cs.User
	}

	return n, nil
}

func marshalNodes(nodes Nodes, ss *stringSet, includeChangeset bool) *osmpb.DenseNodes {
	dense := denseNodesValues(nodes)
	encoded := &osmpb.DenseNodes{
		Ids: encodeInt64(dense.IDs),
		DenseInfo: &osmpb.DenseInfo{
			Versions:   dense.Versions,
			Timestamps: encodeInt64(dense.Timestamps),
			Committeds: encodeInt64(dense.Committeds),
			Visibles:   dense.Visibles,
		},
		Lats: encodeInt64(dense.Lats),
		Lons: encodeInt64(dense.Lons),
	}

	if dense.TagCount > 0 {
		encoded.KeysVals = encodeNodesTags(nodes, ss, dense.TagCount)
	}

	if includeChangeset {
		csinfo := nodesChangesetInfo(nodes, ss)
		encoded.DenseInfo.ChangesetIds = encodeInt64(csinfo.Changesets)
		encoded.DenseInfo.UserIds = encodeInt32(csinfo.UserIDs)
		encoded.DenseInfo.UserSids = encodeInt32(csinfo.UserSids)
	}

	return encoded
}

func unmarshalNodes(encoded *osmpb.DenseNodes, ss []string, cs *Changeset) (Nodes, error) {
	encoded.Ids = decodeInt64(encoded.Ids)
	encoded.Lats = decodeInt64(encoded.Lats)
	encoded.Lons = decodeInt64(encoded.Lons)
	encoded.DenseInfo.Timestamps = decodeInt64(encoded.DenseInfo.Timestamps)
	encoded.DenseInfo.ChangesetIds = decodeInt64(encoded.DenseInfo.ChangesetIds)
	encoded.DenseInfo.Committeds = decodeInt64(encoded.DenseInfo.Committeds)
	encoded.DenseInfo.UserIds = decodeInt32(encoded.DenseInfo.UserIds)
	encoded.DenseInfo.UserSids = decodeInt32(encoded.DenseInfo.UserSids)

	tagLoc := 0
	nodes := make(Nodes, len(encoded.Ids))
	for i := range encoded.Ids {
		n := &Node{
			ID:        NodeID(encoded.Ids[i]),
			Lat:       float64(encoded.Lats[i]) / locMultiple,
			Lon:       float64(encoded.Lons[i]) / locMultiple,
			Visible:   encoded.DenseInfo.Visibles[i],
			Version:   int(encoded.DenseInfo.Versions[i]),
			Timestamp: unixToTime(encoded.DenseInfo.Timestamps[i]),
		}

		if i < len(encoded.DenseInfo.Committeds) {
			n.Committed = unixToTimePointer(encoded.DenseInfo.Committeds[i])
		}

		if cs != nil {
			n.ChangesetID = cs.ID
			n.UserID = cs.UserID
			n.User = cs.User
		} else {
			if len(encoded.DenseInfo.ChangesetIds) > 0 {
				n.ChangesetID = ChangesetID(encoded.DenseInfo.ChangesetIds[i])
			}

			if len(encoded.DenseInfo.UserIds) > 0 {
				n.UserID = UserID(encoded.DenseInfo.UserIds[i])
			}

			if len(encoded.DenseInfo.UserSids) > 0 {
				n.User = ss[encoded.DenseInfo.UserSids[i]]
			}
		}

		if encoded.KeysVals != nil {
			if encoded.KeysVals[tagLoc] == 0 {
				tagLoc++
			} else {
				for encoded.KeysVals[tagLoc] != 0 {
					n.Tags = append(n.Tags, Tag{
						Key:   ss[encoded.KeysVals[tagLoc]],
						Value: ss[encoded.KeysVals[tagLoc+1]],
					})

					tagLoc += 2
				}
				tagLoc++
			}
		}

		nodes[i] = n
	}

	return nodes, nil
}

func marshalWay(way *Way, ss *stringSet, includeChangeset bool) *osmpb.Way {
	keys, vals := way.Tags.keyValues(ss)
	encoded := &osmpb.Way{
		Id:   int64(way.ID),
		Keys: keys,
		Vals: vals,
		Info: &osmpb.Info{
			Version:   int32(way.Version),
			Timestamp: timeToUnix(way.Timestamp),
			Visible:   proto.Bool(way.Visible),
		},
	}

	if way.Committed != nil {
		encoded.Info.Committed = timeToUnixPointer(*way.Committed)
	}

	if len(way.Nodes) > 0 {
		encoded.Refs = encodeWayNodeIDs(way.Nodes)

		if way.Nodes[0].Version != 0 {
			encoded.DenseMembers = encodeDenseWayNodes(way.Nodes)
		}
	}

	encoded.Updates = marshalUpdates(way.Updates, true)

	if includeChangeset {
		encoded.Info.ChangesetId = int64(way.ChangesetID)
		encoded.Info.UserId = int32(way.UserID)
		encoded.Info.UserSid = ss.Add(way.User)
	}

	return encoded
}

func unmarshalWay(encoded *osmpb.Way, ss []string, cs *Changeset) (*Way, error) {
	tags, err := tagsFromStrings(ss, encoded.GetKeys(), encoded.GetVals())
	if err != nil {
		return nil, err
	}

	info := encoded.GetInfo()
	w := &Way{
		ID:          WayID(encoded.GetId()),
		User:        ss[info.GetUserSid()],
		UserID:      UserID(info.GetUserId()),
		Visible:     info.GetVisible(),
		Version:     int(info.GetVersion()),
		ChangesetID: ChangesetID(info.GetChangesetId()),
		Timestamp:   unixToTime(info.GetTimestamp()),
		Committed:   unixToTimePointer(info.GetCommitted()),
		Tags:        tags,
	}

	w.Nodes = decodeWayNodeIDs(encoded.GetRefs())
	decodeDenseWayNodes(w.Nodes, encoded.GetDenseMembers())

	w.Updates = unmarshalUpdates(encoded.GetUpdates())

	if cs != nil {
		w.ChangesetID = cs.ID
		w.UserID = cs.UserID
		w.User = cs.User
	}

	return w, nil
}

func marshalRelation(relation *Relation, ss *stringSet, includeChangeset bool) *osmpb.Relation {
	l := len(relation.Members)
	roles := make([]uint32, l)
	refs := make([]int64, l)
	types := make([]osmpb.Relation_MemberType, l)

	for i, m := range relation.Members {
		roles[i] = ss.Add(m.Role)
		refs[i] = m.Ref
		types[i] = memberTypeMap[m.Type]
	}

	keys, vals := relation.Tags.keyValues(ss)
	encoded := &osmpb.Relation{
		Id:   int64(relation.ID),
		Keys: keys,
		Vals: vals,
		Info: &osmpb.Info{
			Version:   int32(relation.Version),
			Timestamp: timeToUnix(relation.Timestamp),
			Visible:   proto.Bool(relation.Visible),
		},
		Roles: roles,
		Refs:  encodeInt64(refs),
		Types: types,
	}

	if relation.Committed != nil {
		encoded.Info.Committed = timeToUnixPointer(*relation.Committed)
	}

	if len(relation.Members) > 0 && relation.Members[0].Version != 0 {
		encoded.DenseMembers = encodeDenseMembers(relation.Members)
	}

	encoded.Updates = marshalUpdates(relation.Updates, false)

	if includeChangeset {
		encoded.Info.ChangesetId = int64(relation.ChangesetID)
		encoded.Info.UserId = int32(relation.UserID)
		encoded.Info.UserSid = ss.Add(relation.User)
	}

	return encoded
}

func unmarshalRelation(encoded *osmpb.Relation, ss []string, cs *Changeset) (*Relation, error) {
	tags, err := tagsFromStrings(ss, encoded.GetKeys(), encoded.GetVals())
	if err != nil {
		return nil, err
	}

	info := encoded.GetInfo()
	r := &Relation{
		ID:          RelationID(encoded.GetId()),
		User:        ss[info.GetUserSid()],
		UserID:      UserID(info.GetUserId()),
		Visible:     info.GetVisible(),
		Version:     int(info.GetVersion()),
		ChangesetID: ChangesetID(info.GetChangesetId()),
		Timestamp:   unixToTime(info.GetTimestamp()),
		Committed:   unixToTimePointer(info.GetCommitted()),
		Members:     decodeMembers(ss, encoded.GetRoles(), encoded.GetRefs(), encoded.GetTypes()),
		Tags:        tags,
	}

	decodeDenseMembers(r.Members, encoded.GetDenseMembers())
	r.Updates = unmarshalUpdates(encoded.GetUpdates())

	if cs != nil {
		r.ChangesetID = cs.ID
		r.UserID = cs.UserID
		r.User = cs.User
	}

	return r, nil
}

type denseNodesResult struct {
	IDs        []int64
	Lats       []int64
	Lons       []int64
	Timestamps []int64
	Committeds []int64
	Versions   []int32
	Visibles   []bool
	TagCount   int
}

func denseNodesValues(ns Nodes) denseNodesResult {
	l := len(ns)
	ds := denseNodesResult{
		IDs:        make([]int64, l),
		Lats:       make([]int64, l),
		Lons:       make([]int64, l),
		Timestamps: make([]int64, l),
		Committeds: make([]int64, l),
		Versions:   make([]int32, l),
		Visibles:   make([]bool, l),
	}

	cc := 0
	for i, n := range ns {
		ds.IDs[i] = int64(n.ID)
		ds.Lats[i] = geoToInt64(n.Lat)
		ds.Lons[i] = geoToInt64(n.Lon)
		ds.Timestamps[i] = n.Timestamp.Unix()
		ds.Versions[i] = int32(n.Version)
		ds.Visibles[i] = n.Visible
		ds.TagCount += len(n.Tags)

		if n.Committed != nil {
			ds.Committeds[i] = timeToUnix(*n.Committed)
			cc++
		}
	}

	if cc == 0 {
		ds.Committeds = nil
	}

	return ds
}

func encodeNodesTags(ns Nodes, ss *stringSet, count int) []uint32 {
	r := make([]uint32, 0, 2*count+len(ns))
	for _, n := range ns {
		for _, t := range n.Tags {
			r = append(r, ss.Add(t.Key))
			r = append(r, ss.Add(t.Value))
		}
		r = append(r, 0)
	}

	return r
}

type changesetInfoResult struct {
	Changesets []int64
	UserIDs    []int32
	UserSids   []int32
}

func nodesChangesetInfo(ns Nodes, ss *stringSet) changesetInfoResult {
	l := len(ns)
	cs := changesetInfoResult{
		Changesets: make([]int64, l),
		UserIDs:    make([]int32, l),
		UserSids:   make([]int32, l),
	}

	for i, n := range ns {
		cs.Changesets[i] = int64(n.ChangesetID)
		cs.UserIDs[i] = int32(n.UserID)
		cs.UserSids[i] = int32(ss.Add(n.User))
	}

	return cs
}

func encodeWayNodeIDs(waynodes []WayNode) []int64 {
	result := make([]int64, len(waynodes))
	var prev int64

	for i, r := range waynodes {
		result[i] = int64(r.ID) - prev
		prev = int64(r.ID)
	}

	return result
}

func decodeWayNodeIDs(diff []int64) []WayNode {
	if len(diff) == 0 {
		return nil
	}

	result := make([]WayNode, len(diff))
	decodeInt64(diff)

	for i, d := range diff {
		result[i] = WayNode{ID: NodeID(d)}
	}

	return result
}

func encodeDenseWayNodes(waynodes []WayNode) *osmpb.DenseMembers {
	l := len(waynodes)

	versions := make([]int32, l)
	changesetIDs := make([]int64, l)
	lats := make([]int64, l)
	lons := make([]int64, l)

	for i, n := range waynodes {
		lats[i] = geoToInt64(n.Lat)
		lons[i] = geoToInt64(n.Lon)
		versions[i] = int32(n.Version)
		changesetIDs[i] = int64(n.ChangesetID)
	}

	return &osmpb.DenseMembers{
		Versions:     versions,
		ChangesetIds: encodeInt64(changesetIDs),
		Lats:         encodeInt64(lats),
		Lons:         encodeInt64(lons),
	}
}

func decodeDenseWayNodes(waynodes []WayNode, encoded *osmpb.DenseMembers) {
	if encoded == nil {
		return
	}

	decodeInt64(encoded.ChangesetIds)
	decodeInt64(encoded.Lats)
	decodeInt64(encoded.Lons)

	for i := range encoded.Versions {
		waynodes[i].Version = int(encoded.Versions[i])
		waynodes[i].ChangesetID = ChangesetID(encoded.ChangesetIds[i])
		waynodes[i].Lat = float64(encoded.Lats[i]) / locMultiple
		waynodes[i].Lon = float64(encoded.Lons[i]) / locMultiple
	}

	return
}

func decodeMembers(
	ss []string,
	roles []uint32,
	refs []int64,
	types []osmpb.Relation_MemberType,
) []Member {
	if len(roles) == 0 {
		return nil
	}

	result := make([]Member, len(roles))
	decodeInt64(refs)
	for i := range roles {
		result[i].Role = ss[roles[i]]
		result[i].Ref = refs[i]
		result[i].Type = memberTypeMapRev[types[i]]
	}

	return result
}

func encodeDenseMembers(members []Member) *osmpb.DenseMembers {
	l := len(members)
	versions := make([]int32, l)
	changesetIDs := make([]int64, l)
	lats := make([]int64, l)
	lons := make([]int64, l)

	nodes := 0
	for i, m := range members {
		if m.Type == TypeNode {
			nodes++
		}

		lats[i] = geoToInt64(m.Lat)
		lons[i] = geoToInt64(m.Lon)

		versions[i] = int32(m.Version)
		changesetIDs[i] = int64(m.ChangesetID)
	}

	result := &osmpb.DenseMembers{
		Versions:     versions,
		ChangesetIds: encodeInt64(changesetIDs),
	}

	if nodes > 0 {
		result.Lats = encodeInt64(lats)
		result.Lons = encodeInt64(lons)
	}

	return result
}

func decodeDenseMembers(members []Member, encoded *osmpb.DenseMembers) {
	if encoded == nil || len(encoded.Versions) == 0 {
		return
	}

	decodeInt64(encoded.ChangesetIds)
	decodeInt64(encoded.Lats)
	decodeInt64(encoded.Lons)

	for i := range encoded.Versions {
		members[i].Version = int(encoded.Versions[i])
		members[i].ChangesetID = ChangesetID(encoded.ChangesetIds[i])

		if encoded.Lats != nil {
			members[i].Lat = float64(encoded.Lats[i]) / locMultiple
			members[i].Lon = float64(encoded.Lons[i]) / locMultiple
		}
	}

	return
}

func encodeInt32(vals []int32) []int32 {
	var prev int32
	for i, v := range vals {
		vals[i] = v - prev
		prev = v
	}

	return vals
}

func encodeInt64(vals []int64) []int64 {
	var prev int64
	for i, v := range vals {
		vals[i] = v - prev
		prev = v
	}

	return vals
}

func decodeInt32(vals []int32) []int32 {
	var prev int32
	for i, v := range vals {
		prev += v
		vals[i] = prev
	}

	return vals
}

func decodeInt64(vals []int64) []int64 {
	var prev int64
	for i, v := range vals {
		prev += v
		vals[i] = prev
	}

	return vals
}

func geoToInt64(l float64) int64 {
	// on rounding errors
	//
	// It is the case that 32.850314 * 10e6 = 32850313.999999996
	// Simpily casting this as an int will truncate towards zero
	// and result in an off by one. The true solution is to round
	// the scaled result, like so:
	//
	// int64(math.Floor(stream.BaseData[i][0]*factor + 0.5))
	//
	// However, the code below does the same thing in this context,
	// and is twice as fast:
	sign := 0.5
	if l < 0 {
		sign = -0.5
	}

	return int64(l*locMultiple + sign)
}

func timeToUnix(t time.Time) int64 {
	u := t.Unix()
	if u <= 0 {
		return 0
	}

	return u
}

func timeToUnixPointer(t time.Time) *int64 {
	u := t.Unix()
	if u <= 0 {
		return nil
	}

	return &u
}

func unixToTime(u int64) time.Time {
	if u <= 0 {
		return time.Time{}
	}

	return time.Unix(u, 0).UTC()
}

func unixToTimePointer(u int64) *time.Time {
	if u <= 0 {
		return nil
	}

	t := time.Unix(u, 0).UTC()
	return &t
}
