import { Button, Field, Textarea } from '@fluentui/react-components'
import { Events } from '@wailsio/runtime'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import PageHeader from '../components/PageHeader'
import { formatPayload } from '../lib/format'
import { api } from '../lib/api'

export default function AgentView() {
  const navigate = useNavigate()
  const [message, setMessage] = useState('List the active window title and current mouse position.')
  const [messages, setMessages] = useState([])
  const [timeline, setTimeline] = useState([])
  const [responseID, setResponseID] = useState('')
  const [activeRunID, setActiveRunID] = useState('')
  const [status, setStatus] = useState('')
  const [loadingSettings, setLoadingSettings] = useState(true)
  const [hasAPIKey, setHasAPIKey] = useState(false)
  const [apiKeySource, setAPIKeySource] = useState('missing')
  const [previewingCursor, setPreviewingCursor] = useState(false)
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const transcriptRef = useRef(null)
  const transcriptEndRef = useRef(null)
  const pageEndRef = useRef(null)
  const activeRunIDRef = useRef('')

  useEffect(() => {
    async function load() {
      try {
        const settingsStatus = await api.getAgentSettingsStatus()
        setHasAPIKey(Boolean(settingsStatus?.has_api_key))
        setAPIKeySource(settingsStatus?.api_key_source || 'missing')
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
      if (!event || !event.run_id || event.run_id !== activeRunIDRef.current) {
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
  }, [])

  useLayoutEffect(() => {
    const transcript = transcriptRef.current
    const transcriptEnd = transcriptEndRef.current
    if (!transcript || !transcriptEnd) {
      return
    }
    const frameID = requestAnimationFrame(() => {
      transcriptEnd.scrollIntoView({ block: 'end' })
      pageEndRef.current?.scrollIntoView({ block: 'end' })
    })
    return () => {
      cancelAnimationFrame(frameID)
    }
  }, [timeline])

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
    activeRunIDRef.current = clientRunID
    setActiveRunID(clientRunID)

    try {
      const result = await api.chatWithAgent({
        messages: [userMessage],
        previous_response_id: responseID,
        client_run_id: clientRunID
      })
      const assistant = result.message || { role: 'assistant', content: '' }
      setMessages((current) => [...current, assistant])
      setTimeline((current) => appendUniqueTimelineItems(current, [
        ...(result.items || []),
        ...(assistant.content ? [{ kind: 'message', role: 'assistant', content: assistant.content }] : [])
      ]))
      setResponseID(result.response_id || '')
      setStatus('Agent response ready.')
    } catch (nextError) {
      setError(nextError.message || String(nextError))
    } finally {
      setSending(false)
    }
  }

  async function previewAgentCursor() {
    setPreviewingCursor(true)
    setError('')
    try {
      const result = await api.previewAgentCursor()
      setStatus(result?.message || 'Previewing the pink agent cursor overlay on the desktop.')
    } catch (nextError) {
      setError(nextError.message || String(nextError))
    } finally {
      setPreviewingCursor(false)
    }
  }

  return (
    <>
      <PageHeader
        title="Agent"
        description="Run desktop tasks from a focused fullscreen chat workspace."
        actions={
          <Button onClick={() => navigate('/agent-settings')}>
            Open settings
          </Button>
        }
      />

      <div className="gutgd-list gutgd-agentPage gutgd-agentPageFull">
        <section className="gutgd-rowCard gutgd-agentShell">
          <div className="gutgd-rowLead gutgd-agentIntro">
            <div className="gutgd-rowLeadGlyph">◈</div>
            <div>
              <h3>Desktop agent workspace</h3>
              <p>Review the live transcript, watch tool calls stream in, and keep the conversation pinned to the latest result.</p>
            </div>
            <div className="gutgd-agentMetaChips gutgd-agentChatChips">
              <span className={`gutgd-agentChip ${sending ? 'gutgd-agentChip-live' : ''}`}>
                <strong>State</strong>
                <span>{sending ? 'Running' : loadingSettings ? 'Loading' : 'Ready'}</span>
              </span>
              <span className="gutgd-agentChip">
                <strong>Credentials</strong>
                <span>{credentialSourceLabel(apiKeySource)}</span>
              </span>
              {status ? <span className="gutgd-agentChip"><strong>Status</strong><span>{status}</span></span> : null}
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
                  <p>{composeHelperText(hasAPIKey, apiKeySource)}</p>
                </div>
                <div className="gutgd-rowActions">
                  <Button appearance="primary" onClick={sendMessage} disabled={sending || loadingSettings || !hasAPIKey}>
                    {sending ? 'Sending…' : hasAPIKey ? 'Send' : 'API key required'}
                  </Button>
                  {!hasAPIKey ? (
                    <Button onClick={previewAgentCursor} disabled={previewingCursor || loadingSettings}>
                      {previewingCursor ? 'Previewing…' : 'Preview agent cursor'}
                    </Button>
                  ) : null}
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
      <div ref={pageEndRef} aria-hidden="true" />
    </>
  )
}

function appendUniqueTimelineItems(current, incoming) {
  const visibleIncoming = incoming.filter((item) => item.kind === 'message' || item.kind === 'reasoning' || item.kind === 'tool_call' || item.kind === 'tool_output')
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
  if (item.kind === 'reasoning') {
    return 'reasoning'
  }
  if (isContinueMessage(item)) {
    return 'continue'
  }
  return item.role || 'assistant'
}

function renderTranscriptItem(item) {
  if (item.kind === 'tool_call') {
    const details = summarizeToolCall(item)
    const raw = formatPayload(parseStructuredValue(item.arguments, {}))
    return (
      <>
        <header>
          <span>Tool</span>
          <code>{formatToolName(item.name)}</code>
        </header>
        {details.message ? <p>{details.message}</p> : null}
        {details.summary.length ? (
          <ul className="gutgd-chatBulletList">
            {details.summary.map((line) => <li key={line}>{line}</li>)}
          </ul>
        ) : null}
        {renderRawPayload('Show raw request', raw)}
      </>
    )
  }

  if (item.kind === 'tool_output') {
    const details = summarizeToolOutput(item)
    const raw = formatPayload(parseStructuredValue(item.output, item.output || ''))
    return (
      <>
        <header>
          <span>Result</span>
          <code>{formatToolName(item.name)}</code>
        </header>
        <p>{item.error || details.message || 'Completed.'}</p>
        {details.summary.length ? (
          <ul className="gutgd-chatBulletList">
            {details.summary.map((line) => <li key={line}>{line}</li>)}
          </ul>
        ) : null}
        {renderRawPayload('Show raw result', raw)}
      </>
    )
  }

  if (item.kind === 'reasoning') {
    return (
      <>
        <header>
          <span>Thinking</span>
          <code>reasoning</code>
        </header>
        <p>{item.content}</p>
      </>
    )
  }

  if (isContinueMessage(item)) {
    return (
      <>
        <header>
          <span>Continue</span>
          <code>step budget</code>
        </header>
        <p>The agent hit its local step budget and the harness sent <code>continue</code> so it could keep working with the existing conversation state.</p>
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

function renderRawPayload(label, text) {
  if (!text) {
    return null
  }
  return (
    <details className="gutgd-chatRawDetails">
      <summary>{label}</summary>
      <pre className="gutgd-output gutgd-outputCompact gutgd-chatPayload">{text}</pre>
    </details>
  )
}

function summarizeToolCall(item) {
  const args = parseStructuredValue(item.arguments, {})
  if (args == null || typeof args !== 'object' || Array.isArray(args)) {
    return {
      message: '',
      summary: []
    }
  }

  const entries = Object.entries(args)
  return {
    message: entries.length ? `Calling ${formatToolName(item.name)}.` : '',
    summary: entries.map(([key, value]) => `${formatFieldLabel(key)}: ${summarizeValue(value)}`)
  }
}

function summarizeToolOutput(item) {
  const output = parseStructuredValue(item.output, item.output || '')
  if (output == null || typeof output !== 'object' || Array.isArray(output)) {
    return {
      message: typeof output === 'string' ? output : '',
      summary: []
    }
  }

  const preferredMessage = firstNonEmptyString(
    output.message,
    output.analysis,
    output.markdown,
    output.path ? `Saved at ${output.path}.` : ''
  )

  const summary = []
  for (const [key, value] of Object.entries(output)) {
    if (key === 'message' || key === 'analysis' || key === 'markdown') {
      continue
    }
    if (value == null || value === '') {
      continue
    }
    if (typeof value === 'object') {
      summary.push(`${formatFieldLabel(key)}: ${compactJson(value)}`)
      continue
    }
    summary.push(`${formatFieldLabel(key)}: ${summarizeValue(value)}`)
  }

  return {
    message: preferredMessage,
    summary
  }
}

function firstNonEmptyString(...values) {
  for (const value of values) {
    if (typeof value === 'string' && value.trim()) {
      return value.trim()
    }
  }
  return ''
}

function summarizeValue(value) {
  if (typeof value === 'string') {
    const text = value.trim().replaceAll('\n', ' ')
    if (text.length <= 120) {
      return text
    }
    return `${text.slice(0, 117)}...`
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value)
  }
  if (Array.isArray(value)) {
    return compactJson(value)
  }
  if (value && typeof value === 'object') {
    return compactJson(value)
  }
  return String(value)
}

function compactJson(value) {
  const text = JSON.stringify(value)
  if (!text) {
    return ''
  }
  if (text.length <= 120) {
    return text
  }
  return `${text.slice(0, 117)}...`
}

function formatFieldLabel(value) {
  return (value || '').replaceAll('_', ' ')
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
  return isContinueMessage(item)
}

function isMaxDepthAssistantMessage(item) {
  return item?.kind === 'message'
    && item?.role === 'assistant'
    && item?.content === 'The agent reached the maximum tool-call depth before producing a final answer. Review the tool activity above and refine the request if needed.'
}

function isContinueMessage(item) {
  return item?.kind === 'message' && item?.role === 'user' && item?.content === 'continue'
}

function credentialSourceLabel(value) {
  switch (value) {
    case 'saved':
      return 'Saved locally'
    case 'environment':
      return 'OPENAI_API_KEY env'
    default:
      return 'Missing'
  }
}

function composeHelperText(hasAPIKey, apiKeySource) {
  if (apiKeySource === 'environment') {
    return 'Using OPENAI_API_KEY from the environment. You can chat immediately or save a local key in settings if you want this profile to persist its own credentials.'
  }
  if (hasAPIKey) {
    return 'Be specific about the app, target region, and the outcome you want.'
  }
  return 'No API key is configured yet. You can still preview the pink agent cursor overlay from here before wiring the live model.'
}
