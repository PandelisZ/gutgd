import { Button, Field, Textarea } from '@fluentui/react-components'
import { useState } from 'react'

import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import ResultPane from '../components/ResultPane'
import { api } from '../lib/api'

export default function ClipboardView() {
  const [text, setText] = useState('gutgd clipboard smoke test')
  const [output, setOutput] = useState('')

  async function run(task, afterSuccess) {
    try {
      const result = await task()
      setOutput(result)
      if (afterSuccess) {
        afterSuccess(result)
      }
    } catch (error) {
      setOutput({ error: error.message || String(error) })
    }
  }

  return (
    <>
      <PageHeader
        title="Clipboard"
        description="Verify clipboard copy, paste, clear, and has-text behavior through the system provider."
      />

      <div className="gutgd-list">
        <section className="gutgd-rowCard">
          <div className="gutgd-rowLead">
            <div className="gutgd-rowLeadGlyph">⎘</div>
            <div>
              <h3>Clipboard controls</h3>
              <p>Copy, paste, clear, and detect text content through the system clipboard provider.</p>
            </div>
          </div>
          <div className="gutgd-rowBody">
            <Field label="Text">
              <Textarea value={text} onChange={(_, data) => setText(data.value)} />
            </Field>
            <div className="gutgd-rowActions">
              <Button appearance="primary" onClick={() => run(() => api.clipboardCopy({ text }))}>Copy text</Button>
              <Button onClick={() => run(() => api.clipboardPaste(), (result) => setText(result.text || ''))}>Paste text</Button>
              <Button onClick={() => run(() => api.clipboardClear())}>Clear</Button>
              <Button onClick={() => run(() => api.clipboardHasText())}>Has text?</Button>
            </div>
          </div>
        </section>

        <Panel title="Clipboard output" description="Latest clipboard response.">
          <ResultPane value={output} />
        </Panel>
      </div>
    </>
  )
}
