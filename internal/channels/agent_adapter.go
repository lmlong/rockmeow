package channels

import (
	"context"

	"github.com/lingguard/internal/agent"
	"github.com/lingguard/pkg/stream"
)

// AgentAdapter 将 Channel 消息转发给 Agent
type AgentAdapter struct {
	agent *agent.Agent
}

// NewAgentAdapter 创建新的 Agent 适配器
func NewAgentAdapter(ag *agent.Agent) *AgentAdapter {
	return &AgentAdapter{agent: ag}
}

// HandleMessage 实现 MessageHandler 接口
func (a *AgentAdapter) HandleMessage(ctx context.Context, msg *Message) (string, error) {
	return a.agent.ProcessMessage(ctx, msg.SessionID, msg.Content)
}

// HandleMessageStream 实现 StreamingMessageHandler 接口
func (a *AgentAdapter) HandleMessageStream(ctx context.Context, msg *Message, callback stream.StreamCallback) error {
	return a.agent.ProcessMessageStream(ctx, msg.SessionID, msg.Content, callback)
}
