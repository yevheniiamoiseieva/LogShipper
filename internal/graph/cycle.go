package graph

type color int

const (
	white color = iota
	gray
	black
)

type cycleDetector struct {
	knownCycles map[string]bool
}

func newCycleDetector() *cycleDetector {
	return &cycleDetector{
		knownCycles: make(map[string]bool),
	}
}

func (cd *cycleDetector) findNewCycles(adjacency map[NodeID][]NodeID) [][]NodeID {
	colors := make(map[NodeID]color, len(adjacency))
	parent := make(map[NodeID]NodeID, len(adjacency))

	var result [][]NodeID

	for node := range adjacency {
		if colors[node] == white {
			cd.dfs(node, adjacency, colors, parent, &result)
		}
	}

	return result
}

func (cd *cycleDetector) dfs(
	v NodeID,
	adj map[NodeID][]NodeID,
	colors map[NodeID]color,
	parent map[NodeID]NodeID,
	result *[][]NodeID,
) {
	colors[v] = gray

	for _, u := range adj[v] {
		switch colors[u] {
		case gray:
			cycle := cd.extractCycle(v, u, parent)
			key := cycleKey(cycle)
			if !cd.knownCycles[key] {
				cd.knownCycles[key] = true
				*result = append(*result, cycle)
			}
		case white:
			parent[u] = v
			cd.dfs(u, adj, colors, parent, result)
		}
	}

	colors[v] = black
}

func (cd *cycleDetector) extractCycle(backSrc, backDst NodeID, parent map[NodeID]NodeID) []NodeID {
	path := []NodeID{backSrc}
	cur := backSrc
	for cur != backDst {
		p, ok := parent[cur]
		if !ok {
			break
		}
		path = append(path, p)
		cur = p
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	path = append(path, backDst)
	return path
}

func cycleKey(cycle []NodeID) string {
	if len(cycle) == 0 {
		return ""
	}
	nodes := cycle
	if len(nodes) > 1 && nodes[0] == nodes[len(nodes)-1] {
		nodes = nodes[:len(nodes)-1]
	}

	minIdx := 0
	for i := 1; i < len(nodes); i++ {
		if nodes[i] < nodes[minIdx] {
			minIdx = i
		}
	}

	key := make([]byte, 0, 64)
	for i := 0; i < len(nodes); i++ {
		if i > 0 {
			key = append(key, '|')
		}
		key = append(key, nodes[(minIdx+i)%len(nodes)]...)
	}
	return string(key)
}
