import { Button, Field, Input, Textarea } from '@fluentui/react-components'
import { useState } from 'react'

import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import ResultPane from '../components/ResultPane'
import { api } from '../lib/api'

export default function KeyboardView() {
  const [text, setText] = useState('Hello from gutgd')
  const [delay, setDelay] = useState('50')
  const [keys, setKeys] = useState('ctrl,c')
  const [output, setOutput] = useState('')

  async function run(task) {
    try {
      const result = await task()
      setOutput(result)
    } catch (error) {
      setOutput({ error: error.message || String(error) })
    }
  }

  const keyPayload = {
    keys: keys.split(',').map((value) => value.trim()).filter(Boolean),
    auto_delay_ms: Number(delay)
  }

  return (
    <>
      <PageHeader
        title="Keyboard"
        description="Type text, press key combinations, and verify keyboard automation behavior against a live desktop target."
      />

      <div className="gutgd-list">
        <section className="gutgd-rowCard">
          <div className="gutgd-rowLead">
            <div className="gutgd-rowLeadGlyph">⌨</div>
            <div>
              <h3>Type text</h3>
              <p>Send plain text into the focused application using the live keyboard provider.</p>
            </div>
          </div>
          <div className="gutgd-rowBody">
            <Field label="Text">
              <Textarea value={text} onChange={(_, data) => setText(data.value)} />
            </Field>
            <Field label="Auto delay (ms)">
              <Input type="number" value={delay} onChange={(_, data) => setDelay(data.value)} />
            </Field>
            <div className="gutgd-rowActions">
              <Button appearance="primary" onClick={() => run(() => api.typeText({ text, auto_delay_ms: Number(delay) }))}>
                Type text
              </Button>
            </div>
          </div>
        </section>

        <section className="gutgd-rowCard">
          <div className="gutgd-rowLead">
            <div className="gutgd-rowLeadGlyph">⇧</div>
            <div>
              <h3>Tap or hold keys</h3>
              <p>Drive key sequences such as <code>ctrl,c</code>, <code>enter</code>, or <code>leftwin,r</code>.</p>
            </div>
          </div>
          <div className="gutgd-rowBody">
            <Field label="Key list">
              <Input value={keys} onChange={(_, data) => setKeys(data.value)} />
            </Field>
            <div className="gutgd-rowActions">
              <Button appearance="primary" onClick={() => run(() => api.tapKeys(keyPayload))}>Tap keys</Button>
              <Button onClick={() => run(() => api.pressKeys(keyPayload))}>Press keys</Button>
              <Button onClick={() => run(() => api.releaseKeys(keyPayload))}>Release keys</Button>
            </div>
          </div>
        </section>

        <Panel title="Keyboard output" description="Latest action result or backend error.">
          <ResultPane value={output} />
        </Panel>
      </div>
    </>
  )
}
