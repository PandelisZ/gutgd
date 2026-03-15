import { Button, Field, Input, Text } from '@fluentui/react-components'
import { useMemo, useState } from 'react'

import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import ResultPane from '../components/ResultPane'
import { api } from '../lib/api'

export default function ScreenView() {
  const [captureName, setCaptureName] = useState('gutgd-capture.png')
  const [left, setLeft] = useState('100')
  const [top, setTop] = useState('100')
  const [width, setWidth] = useState('400')
  const [height, setHeight] = useState('250')
  const [colorX, setColorX] = useState('100')
  const [colorY, setColorY] = useState('100')
  const [colorResult, setColorResult] = useState(null)
  const [output, setOutput] = useState('')

  const region = useMemo(() => ({
    left: Number(left),
    top: Number(top),
    width: Number(width),
    height: Number(height)
  }), [left, top, width, height])

  async function run(task) {
    try {
      const result = await task()
      setOutput(result)
      return result
    } catch (error) {
      const payload = { error: error.message || String(error) }
      setOutput(payload)
      throw error
    }
  }

  return (
    <>
      <PageHeader
        title="Screen"
        description="Capture the screen, highlight regions, and inspect pixel colors to verify screen primitives."
      />

      <div className="gutgd-list">
        <section className="gutgd-rowCard">
          <div className="gutgd-rowLead">
            <div className="gutgd-rowLeadGlyph">▣</div>
            <div>
              <h3>Screen inspection</h3>
              <p>Capture the full screen or a region, and trigger native highlights.</p>
            </div>
          </div>
          <div className="gutgd-rowBody">
            <Field label="Capture file name">
              <Input value={captureName} onChange={(_, data) => setCaptureName(data.value)} />
            </Field>
            <div className="gutgd-rowActions">
              <Button onClick={() => run(() => api.getScreenSize())}>Read size</Button>
              <Button appearance="primary" onClick={() => run(() => api.captureScreen({ file_name: captureName }))}>Capture full screen</Button>
            </div>
            <div className="gutgd-grid4">
              <Field label="Left"><Input type="number" value={left} onChange={(_, data) => setLeft(data.value)} /></Field>
              <Field label="Top"><Input type="number" value={top} onChange={(_, data) => setTop(data.value)} /></Field>
              <Field label="Width"><Input type="number" value={width} onChange={(_, data) => setWidth(data.value)} /></Field>
              <Field label="Height"><Input type="number" value={height} onChange={(_, data) => setHeight(data.value)} /></Field>
            </div>
            <div className="gutgd-rowActions">
              <Button onClick={() => run(() => api.captureRegion({ file_name: captureName, region }))}>Capture region</Button>
              <Button onClick={() => run(() => api.highlightRegion(region))}>Highlight region</Button>
            </div>
          </div>
        </section>

        <section className="gutgd-rowCard">
          <div className="gutgd-rowLead">
            <div className="gutgd-rowLeadGlyph">◌</div>
            <div>
              <h3>Color at point</h3>
              <p>Grab the pixel color at a coordinate and display the resolved RGBA value.</p>
            </div>
          </div>
          <div className="gutgd-rowBody">
            <div className="gutgd-grid2">
              <Field label="X"><Input type="number" value={colorX} onChange={(_, data) => setColorX(data.value)} /></Field>
              <Field label="Y"><Input type="number" value={colorY} onChange={(_, data) => setColorY(data.value)} /></Field>
            </div>
            <div className="gutgd-rowActions">
              <Button
                appearance="primary"
                onClick={async () => {
                  const result = await run(() => api.colorAt({ x: Number(colorX), y: Number(colorY) }))
                  setColorResult(result)
                }}
              >
                Read color
              </Button>
            </div>
            <div className="gutgd-colorSwatch" style={{ background: colorResult?.color?.hex || 'rgba(255,255,255,0.03)' }}>
              <Text weight="semibold">{colorResult?.color?.hex || 'No color loaded'}</Text>
            </div>
          </div>
        </section>

        <Panel title="Screen output" description="Latest capture or inspection result.">
          <ResultPane value={output} />
        </Panel>
      </div>
    </>
  )
}
