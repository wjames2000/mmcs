package minutes

// ChainNode 推理链中的单个节点
type ChainNode struct {
	Name     string      `json:"name"`
	RoleName string      `json:"role_name,omitempty"`
	Content  string      `json:"content,omitempty"`
	Children []ChainNode `json:"children,omitempty"`
}

// ReasoningChain 完整的推理路径
// 从所有节点执行记录中提取，展示讨论的逻辑演进
type ReasoningChain struct {
	Nodes []ChainNode `json:"nodes"`
}

// BuildChain 从 CallbackRecord 数组构建完整的推理链
// 按执行顺序组织节点，形成树状推理路径
func BuildChain(records []CallbackRecord) ReasoningChain {
	if len(records) == 0 {
		return ReasoningChain{Nodes: []ChainNode{}}
	}

	nodes := make([]ChainNode, 0, len(records))
	var currentRound int
	var roundRoot *ChainNode

	for _, rec := range records {
		node := ChainNode{
			Name:     rec.NodeName,
			RoleName: rec.RoleName,
			Content:  rec.Output,
		}

		// 轮次开始，创建新的根节点
		if rec.NodeName == "round_start" || rec.NodeName == "author_statement" {
			if roundRoot != nil {
				nodes = append(nodes, *roundRoot)
			}
			roundRoot = &ChainNode{
				Name:    rec.NodeName,
				Content: rec.Input,
			}
			currentRound = rec.Round
			continue
		}

		// 发言节点作为当前轮的子节点
		if roundRoot != nil && (rec.Round == currentRound || currentRound == 0) {
			roundRoot.Children = append(roundRoot.Children, node)
		} else {
			// 没有轮次上下文的节点
			if roundRoot != nil {
				nodes = append(nodes, *roundRoot)
			}
			roundRoot = nil
			nodes = append(nodes, node)
		}
	}

	// 追加最后一个轮次
	if roundRoot != nil {
		nodes = append(nodes, *roundRoot)
	}

	return ReasoningChain{Nodes: nodes}
}
