import { Button, Field, Input, Textarea } from '@fluentui/react-components'
import { Events } from '@wailsio/runtime'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'

import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import { formatPayload } from '../lib/format'
import { api } from '../lib/api'

export default function AgentView() {
  const [apiKey, setAPIKey] = useState('')
  const [model, setModel] = useState('gpt-5.4')
  const [models, setModels] = useState([])
  const [reasoningEffort, setReasoningEffort] = useState('medium')
  const [systemPrompt, setSystemPrompt] = useState('')
  const [message, setMessage] = useState('List the active window title and current mouse position.')
  const [messages, setMessages] = useState([])
  const [timeline, setTimeline] = useState([])
  const [responseID, setResponseID] = useState('')
  const [activeRunID, setActiveRunID] = useState('')
  const [status, setStatus] = useState('')
  const [loadingSettings, setLoadingSettings] = useState(true)
  const [savingSettings, setSavingSettings] = useState(false)
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const transcriptRef = useRef(null)
  const transcriptEndRef = useRef(null)

  useEffect(() => {
    async function load() {
      try {
        const settings = await api.getAgentSettings()
        setAPIKey(settings.api_key || '')
        setModel(settings.model || 'gpt-5.4')
        setReasoningEffort(settings.reasoning_effort || 'medium')
        setSystemPrompt(settings.system_prompt || '')
        const options = await api.listAgentModels()
        setModels(options || [])
      } catch (nextError) {
        setError(nextError.message || String(nextError))
      } finally {
        setLoadingSettings(false)
      }
    }

    load()
  }, [])

  useEffect(() => {
    const off = Events.On('agent_progress', (wailsEvent) => {
      const event = wailsEvent?.data || wailsEvent
      if (!event || !event.run_id || event.run_id !== activeRunID) {
        return
      }
      if (event.status) {
        setStatus(event.status)
      }
      if (event.item) {
        setTimeline((current) => appendUniqueTimelineItems(current, [event.item]))
      }
      if (event.kind === 'complete') {
        setResponseID(event.response_id || '')
        setSending(false)
      }
    })
    return () => {
      off?.()
    }
  }, [activeRunID])

  useLayoutEffect(() => {
    const transcript = transcriptRef.current
    const transcriptEnd = transcriptEndRef.current
    if (!transcript || !transcriptEnd) {
      return
    }
    const frameID = requestAnimationFrame(() => {
      transcriptEnd.scrollIntoView({ block: 'end' })
    })
    return () => {
      cancelAnimationFrame(frameID)
    }
  }, [timeline])

  async function saveSettings() {
    setSavingSettings(true)
    setError('')
    setStatus('')
    try {
      const saved = await api.saveAgentSettings({
        api_key: apiKey,
        model,
        reasoning_effort: reasoningEffort,
        system_prompt: systemPrompt
      })
      setAPIKey(saved.api_key || '')
      setModel(saved.model || 'gpt-5.4')
      setReasoningEffort(saved.reasoning_effort || 'medium')
      setSystemPrompt(saved.system_prompt || '')
      setStatus('Settings saved.')
    } catch (nextError) {
      setError(nextError.message || String(nextError))
    } finally {
      setSavingSettings(false)
    }
  }

  async function sendMessage() {
    const content = message.trim()
    if (!content) {
      return
    }

    const userMessage = { role: 'user', content }
    setMessages((current) => [...current, userMessage])
    setTimeline((current) => [...current, { kind: 'message', role: 'user', content }])
    setMessage('')
    setSending(true)
    setError('')
    setStatus('')
    const clientRunID = `agent-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    setActiveRunID(clientRunID)

    try {
      const payloadMessages = responseID ? [userMessage] : [...messages, userMessage]
      const result = await api.chatWithAgent({
        messages: payloadMessages,
        previous_response_id: responseID,
        client_run_id: clientRunID
      })
      const assistant = result.message || { role: 'assistant', content: '' }
      setMessages((current) => [...current, assistant])
      setTimeline((current) => appendUniqueTimelineItems(current, result.items || []))
      setResponseID(result.response_id || '')
      setStatus('Agent response ready.')
    } catch (nextError) {
      setError(nextError.message || String(nextError))
    } finally {
      setSending(false)
    }
  }

  return (
    <>
      <PageHeader
        title="Agent"
        description="Configure the desktop agent, inspect its live tool activity, and run desktop tasks from a focused chat workspace."
        actions={
          <Button appearance="primary" onClick={saveSettings} disabled={savingSettings || loadingSettings}>
            {savingSettings ? 'Saving…' : 'Save settings'}
          </Button>
        }
      />

      <div className="gutgd-list gutgd-agentPage">
        <Panel
          className="gutgd-agentSettingsCard"
          title="OpenAI settings"
          description="These settings are stored locally for this desktop app profile."
        >
          <div className="gutgd-grid2">
            <Field label="API key">
              <Input type="password" value={apiKey} onChange={(_, data) => setAPIKey(data.value)} />
            </Field>
            <Field label="Model">
              <select className="gutgd-nativeSelect" value={model} onChange={(event) => setModel(event.target.value)}>
                {models.map((item) => (
                  <option key={item.id} value={item.id}>{item.id}</option>
                ))}
                {!models.some((item) => item.id === model) ? <option value={model}>{model}</option> : null}
              </select>
            </Field>
          </div>
          <div className="gutgd-grid2">
            <Field label="Reasoning effort">
              <select className="gutgd-nativeSelect" value={reasoningEffort} onChange={(event) => setReasoningEffort(event.target.value)}>
                <option value="none">none</option>
                <option value="minimal">minimal</option>
                <option value="low">low</option>
                <option value="medium">medium</option>
                <option value="high">high</option>
                <option value="xhigh">xhigh</option>
              </select>
            </Field>
            <Field label="System prompt">
              <Textarea value={systemPrompt} onChange={(_, data) => setSystemPrompt(data.value)} />
            </Field>
          </div>
          <div className="gutgd-agentFeedback">
            <div className="gutgd-agentMetaChips">
              <span className="gutgd-agentChip">
                <strong>Model</strong>
                <span>{model}</span>
              </span>
              <span className="gutgd-agentChip">
                <strong>Reasoning</strong>
                <span>{reasoningEffort}</span>
              </span>
              <span className={`gutgd-agentChip ${sending ? 'gutgd-agentChip-live' : ''}`}>
                <strong>State</strong>
                <span>{sending ? 'Running' : loadingSettings ? 'Loading' : 'Ready'}</span>
              </span>
            </div>
            {status ? <div className="gutgd-statusNote gutgd-agentNotice">{status}</div> : null}
            {error ? <div className="gutgd-errorNote gutgd-agentNotice gutgd-agentErrorBlock">{error}</div> : null}
          </div>
        </Panel>

        <section className="gutgd-rowCard gutgd-agentShell">
          <div className="gutgd-rowLead gutgd-agentIntro">
            <div className="gutgd-rowLeadGlyph">◈</div>
            <div>
              <h3>Tool-calling chat</h3>
              <p>Review the live transcript, watch tool calls stream in, and keep the conversation pinned to the latest result.</p>
            </div>
            <div className="gutgd-agentMetaChips gutgd-agentChatChips">
              <span className="gutgd-agentChip">
                <strong>Messages</strong>
                <span>{timeline.length}</span>
              </span>
              <span className="gutgd-agentChip">
                <strong>Session</strong>
                <span>{responseID ? 'continued' : 'new'}</span>
              </span>
            </div>
          </div>

          <div className="gutgd-rowBody gutgd-agentBody">
            <div ref={transcriptRef} className="gutgd-chatTranscript" aria-label="Conversation transcript">
              {timeline.map((item, index) => (
                <article key={`${item.kind}-${item.call_id || item.role || item.name}-${index}`} className={`gutgd-chatMessage gutgd-chatMessage-${messageTone(item)}`}>
                  {renderTranscriptItem(item)}
                </article>
              ))}
              {!timeline.length ? (
                <div className="gutgd-empty gutgd-chatEmptyState">
                  <div className="gutgd-chatEmptyGlyph">◈</div>
                  <div>
                    <strong>Ready for a desktop task</strong>
                    <p>Ask the agent to inspect the active window, capture a region, or carry out a multi-step desktop workflow.</p>
                  </div>
                </div>
              ) : null}
              <div ref={transcriptEndRef} aria-hidden="true" />
            </div>

            <div className="gutgd-chatComposer">
              <div className="gutgd-chatComposerHeader">
                <div>
                  <strong>Compose</strong>
                  <p>Be specific about the app, target region, and the outcome you want.</p>
                </div>
                <div className="gutgd-rowActions">
                  <Button appearance="primary" onClick={sendMessage} disabled={sending || loadingSettings}>
                    {sending ? 'Sending…' : 'Send'}
                  </Button>
                  <Button onClick={() => {
                    setMessages([])
                    setTimeline([])
                    setResponseID('')
                    setActiveRunID('')
                    setError('')
                    setStatus('')
                  }}>
                    Clear chat
                  </Button>
                </div>
              </div>
              <Field label="Message">
                <Textarea
                  placeholder="Open Slack, find the latest message from Alex, and summarize it."
                  value={message}
                  onChange={(_, data) => setMessage(data.value)}
                />
              </Field>
            </div>
          </div>
        </section>
      </div>
    </>
  )
}

function appendUniqueTimelineItems(current, incoming) {
  const visibleIncoming = incoming.filter((item) => item.kind === 'message' || item.kind === 'tool_call' || item.kind === 'tool_output')
  const seen = new Set(current.map(timelineKey))
  const next = [...current]
  for (const item of visibleIncoming) {
    const key = timelineKey(item)
    if (seen.has(key)) {
      continue
    }
    seen.add(key)
    next.push(item)
  }
  return normalizeTimelineOrder(next)
}

function timelineKey(item) {
  return [
    item.kind || '',
    item.call_id || '',
    item.role || '',
    item.name || '',
    item.content || '',
    item.arguments || '',
    item.output || '',
    item.error || ''
  ].join(':')
}

function messageTone(item) {
  if (item.kind === 'tool_call' || item.kind === 'tool_output') {
    return 'tool'
  }
  return item.role || 'assistant'
}

function renderTranscriptItem(item) {
  if (item.kind === 'tool_call') {
    return (
      <>
        <header>
          <span>tool</span>
          <code>{formatToolName(item.name)}</code>
        </header>
        {renderPayload('args', item.arguments, {})}
      </>
    )
  }

  if (item.kind === 'tool_output') {
    return (
      <>
        <header>
          <span>result</span>
          <code>{formatToolName(item.name)}</code>
        </header>
        {item.error ? <p>{item.error}</p> : null}
        {renderPayload('output', item.output, item.output || '')}
      </>
    )
  }

  return (
    <>
      <header>{item.role}</header>
      <p>{item.content}</p>
    </>
  )
}

function parseStructuredValue(value, fallback) {
  if (!value) {
    return fallback
  }
  try {
    return JSON.parse(value)
  } catch {
    return value
  }
}

function formatToolName(value) {
  return (value || '').replaceAll('_', ' ')
}

function renderPayload(label, value, fallback) {
  const text = formatPayload(parseStructuredValue(value, fallback))
  if (!text) {
    return null
  }
  if (text.length <= 120 && !text.includes('\n')) {
    return (
      <div className="gutgd-chatMetaRow">
        <span className="gutgd-chatMetaLabel">{label}</span>
        <code className="gutgd-chatInlinePayload">{text}</code>
      </div>
    )
  }
  return (
    <div className="gutgd-chatMetaBlock">
      <span className="gutgd-chatMetaLabel">{label}</span>
      <pre className="gutgd-output gutgd-outputCompact gutgd-chatPayload">{text}</pre>
    </div>
  )
}

function normalizeTimelineOrder(items) {
  if (items.length < 2) {
    return items
  }

  const next = [...items]
  for (let index = 1; index < next.length; index++) {
    const previous = next[index - 1]
    const current = next[index]
    if (!isTrailingContinueMessage(previous) || !isMaxDepthAssistantMessage(current)) {
      continue
    }
    next[index - 1] = current
    next[index] = previous
  }
  return next
}

function isTrailingContinueMessage(item) {
  return item?.kind === 'message' && item?.role === 'user' && item?.content === 'continue'
}

function isMaxDepthAssistantMessage(item) {
  return item?.kind === 'message'
    && item?.role === 'assistant'
    && item?.content === 'The agent reached the maximum tool-call depth before producing a final answer. Review the tool activity above and refine the request if needed.'
}
