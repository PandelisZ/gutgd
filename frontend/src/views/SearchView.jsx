import { Button, Checkbox, Field, Input } from '@fluentui/react-components'
import { useState } from 'react'

import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import ResultPane from '../components/ResultPane'
import { api } from '../lib/api'

export default function SearchView() {
  const [r, setR] = useState('255')
  const [g, setG] = useState('255')
  const [b, setB] = useState('255')
  const [a, setA] = useState('255')
  const [timeout, setTimeoutValue] = useState('5000')
  const [interval, setIntervalValue] = useState('250')
  const [title, setTitle] = useState('Calculator')
  const [useRegex, setUseRegex] = useState(false)
  const [output, setOutput] = useState('')

  const colorPayload = {
    r: Number(r),
    g: Number(g),
    b: Number(b),
    a: Number(a),
    timeout_ms: Number(timeout),
    interval_ms: Number(interval)
  }

  const windowPayload = {
    title,
    use_regex: useRegex,
    timeout_ms: Number(timeout),
    interval_ms: Number(interval)
  }

  async function run(task) {
    try {
      const result = await task()
      setOutput(result)
    } catch (error) {
      setOutput({ error: error.message || String(error) })
    }
  }

  return (
    <>
      <PageHeader
        title="Search + Assert"
        description="Exercise color and window query flows, including wait and assert behaviors, from one place."
      />

      <div className="gutgd-list">
        <section className="gutgd-rowCard">
          <div className="gutgd-rowLead">
            <div className="gutgd-rowLeadGlyph">⌕</div>
            <div>
              <h3>Color query</h3>
              <p>Find, wait for, or assert a color in the visible screen region.</p>
            </div>
          </div>
          <div className="gutgd-rowBody">
            <div className="gutgd-grid4">
              <Field label="R"><Input type="number" value={r} onChange={(_, data) => setR(data.value)} /></Field>
              <Field label="G"><Input type="number" value={g} onChange={(_, data) => setG(data.value)} /></Field>
              <Field label="B"><Input type="number" value={b} onChange={(_, data) => setB(data.value)} /></Field>
              <Field label="A"><Input type="number" value={a} onChange={(_, data) => setA(data.value)} /></Field>
            </div>
            <div className="gutgd-grid2">
              <Field label="Timeout (ms)"><Input type="number" value={timeout} onChange={(_, data) => setTimeoutValue(data.value)} /></Field>
              <Field label="Interval (ms)"><Input type="number" value={interval} onChange={(_, data) => setIntervalValue(data.value)} /></Field>
            </div>
            <div className="gutgd-rowActions">
              <Button appearance="primary" onClick={() => run(() => api.findColor(colorPayload))}>Find color</Button>
              <Button onClick={() => run(() => api.waitForColor(colorPayload))}>Wait for color</Button>
              <Button onClick={() => run(() => api.assertColorVisible(colorPayload))}>Assert visible</Button>
            </div>
          </div>
        </section>

        <section className="gutgd-rowCard">
          <div className="gutgd-rowLead">
            <div className="gutgd-rowLeadGlyph">✦</div>
            <div>
              <h3>Window query</h3>
              <p>Find or wait for a title match. Regex matching is optional.</p>
            </div>
          </div>
          <div className="gutgd-rowBody">
            <Field label="Window title">
              <Input value={title} onChange={(_, data) => setTitle(data.value)} />
            </Field>
            <Checkbox checked={useRegex} label="Treat title as regular expression" onChange={(_, data) => setUseRegex(Boolean(data.checked))} />
            <div className="gutgd-rowActions">
              <Button appearance="primary" onClick={() => run(() => api.findWindowByTitle(windowPayload))}>Find window</Button>
              <Button onClick={() => run(() => api.waitForWindowByTitle(windowPayload))}>Wait for window</Button>
              <Button onClick={() => run(() => api.assertWindowVisible(windowPayload))}>Assert visible</Button>
            </div>
          </div>
        </section>

        <Panel title="Search output" description="Latest query result or assertion failure.">
          <ResultPane value={output} />
        </Panel>
      </div>
    </>
  )
}
