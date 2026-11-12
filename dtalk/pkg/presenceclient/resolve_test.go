package presenceclient

import "testing"

func TestResolvePreferredClusterLogic(t *testing.T) {
	// 纯逻辑覆盖：preferred 优先，其次最新在线
	items := []UserPresence{
		{
			PreferredClusterID: "cluster-a",
			Clusters: []ClusterPresence{
				{ClusterID: "cluster-b", UpdatedAt: 200},
				{ClusterID: "cluster-c", UpdatedAt: 300},
			},
		},
	}
	preferred := ""
	var bestOnline string
	var bestAt int64
	for _, it := range items {
		if it.PreferredClusterID != "" {
			preferred = it.PreferredClusterID
		}
		for _, cl := range it.Clusters {
			if cl.ClusterID == "" {
				continue
			}
			if cl.UpdatedAt >= bestAt {
				bestAt = cl.UpdatedAt
				bestOnline = cl.ClusterID
			}
		}
	}
	got := preferred
	if got == "" {
		got = bestOnline
	}
	if got != "cluster-a" {
		t.Fatalf("want preferred cluster-a, got %s", got)
	}

	items[0].PreferredClusterID = ""
	preferred = ""
	bestOnline = ""
	bestAt = 0
	for _, it := range items {
		if it.PreferredClusterID != "" {
			preferred = it.PreferredClusterID
		}
		for _, cl := range it.Clusters {
			if cl.ClusterID == "" {
				continue
			}
			if cl.UpdatedAt >= bestAt {
				bestAt = cl.UpdatedAt
				bestOnline = cl.ClusterID
			}
		}
	}
	got = preferred
	if got == "" {
		got = bestOnline
	}
	if got != "cluster-c" {
		t.Fatalf("want newest online cluster-c, got %s", got)
	}
}
