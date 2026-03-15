import { Button, Field, Input, Textarea } from '@fluentui/react-components'
import { useEffect, useState } from 'react'

import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import ResultPane from '../components/ResultPane'
import { api } from '../lib/api'

export default function AgentView() {
  const [apiKey, setAPIKey] = useState('')
  const [model, setModel] = useState('gpt-5.4')
  const [message, setMessage] = useState('List the active window title and current mouse position.')
  const [messages, setMessages] = useState([])
  const [toolEvents, setToolEvents] = useState([])
  const [status, setStatus] = useState('')
  const [loadingSettings, setLoadingSettings] = useState(true)
  const [savingSettings, setSavingSettings] = useState(false)
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const [rawResponse, setRawResponse] = useState('')

  useEffect(() => {
    async function load() {
      try {
        const settings = await api.getAgentSettings()
        setAPIKey(settings.api_key || '')
        setModel(settings.model || 'gpt-5.4')
      } catch (nextError) {
        setError(nextError.message || String(nextError))
      } finally {
        setLoadingSettings(false)
      }
    }

    load()
  }, [])

  async function saveSettings() {
    setSavingSettings(true)
    setError('')
    setStatus('')
    try {
      const saved = await api.saveAgentSettings({
        api_key: apiKey,
        model
      })
      setAPIKey(saved.api_key || '')
      setModel(saved.model || 'gpt-5.4')
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

    const nextMessages = [...messages, { role: 'user', content }]
    setMessages(nextMessages)
    setMessage('')
    setSending(true)
    setError('')
    setStatus('')

    try {
      const result = await api.chatWithAgent({ messages: nextMessages })
      const assistant = result.message || { role: 'assistant', content: '' }
      setMessages([...nextMessages, assistant])
      setToolEvents(result.tool_events || [])
      setRawResponse(result)
    } catch (nextError) {
      setMessages(nextMessages)
      setError(nextError.message || String(nextError))
    } finally {
      setSending(false)
    }
  }

  return (
    <>
      <PageHeader
        title="Agent"
        description="Save an OpenAI API key locally, choose a model, and test a Responses API agent that can call the live gut desktop tools."
        actions={
          <Button appearance="primary" onClick={saveSettings} disabled={savingSettings || loadingSettings}>
            {savingSettings ? 'Saving…' : 'Save settings'}
          </Button>
        }
      />

      <div className="gutgd-list">
        <Panel title="OpenAI settings" description="These settings are stored locally for this desktop app profile.">
          <div className="gutgd-grid2">
            <Field label="API key">
              <Input type="password" value={apiKey} onChange={(_, data) => setAPIKey(data.value)} />
            </Field>
            <Field label="Model">
              <Input value={model} onChange={(_, data) => setModel(data.value)} />
            </Field>
          </div>
          {status ? <div className="gutgd-statusNote">{status}</div> : null}
          {error ? <div className="gutgd-errorNote">{error}</div> : null}
        </Panel>

        <section className="gutgd-rowCard gutgd-agentShell">
          <div className="gutgd-rowLead">
            <div className="gutgd-rowLeadGlyph">◈</div>
            <div>
              <h3>Tool-calling chat</h3>
              <p>Chat with a model that can inspect and drive the same live desktop tools exposed elsewhere in gutgd.</p>
            </div>
          </div>

          <div className="gutgd-rowBody">
            <div className="gutgd-chatTranscript" aria-label="Conversation transcript">
              {messages.map((item, index) => (
                <article key={`${item.role}-${index}`} className={`gutgd-chatMessage gutgd-chatMessage-${item.role}`}>
                  <header>{item.role}</header>
                  <p>{item.content}</p>
                </article>
              ))}
              {!messages.length ? <div className="gutgd-empty">No messages yet.</div> : null}
            </div>

            <Field label="Message">
              <Textarea value={message} onChange={(_, data) => setMessage(data.value)} />
            </Field>

            <div className="gutgd-rowActions">
              <Button appearance="primary" onClick={sendMessage} disabled={sending || loadingSettings}>
                {sending ? 'Sending…' : 'Send'}
              </Button>
              <Button onClick={() => {
                setMessages([])
                setToolEvents([])
                setRawResponse('')
                setError('')
                setStatus('')
              }}>
                Clear chat
              </Button>
            </div>
          </div>
        </section>

        <Panel title="Tool events" description="Latest tool calls and outputs from the model run.">
          <div className="gutgd-featureList">
            {toolEvents.map((item) => (
              <article key={`${item.call_id}-${item.name}`} className="gutgd-featureItem">
                <header>
                  <strong>{item.name}</strong>
                  <span>{item.call_id}</span>
                </header>
                <p><strong>Arguments:</strong> {item.arguments || '{}'}</p>
                {item.error ? <p><strong>Error:</strong> {item.error}</p> : null}
                <pre className="gutgd-output">{item.output || ''}</pre>
              </article>
            ))}
            {!toolEvents.length ? <div className="gutgd-empty">No tool calls yet.</div> : null}
          </div>
        </Panel>

        <Panel title="Raw response" description="Latest assistant response payload returned by the backend.">
          <ResultPane value={rawResponse} />
        </Panel>
      </div>
    </>
  )
}
