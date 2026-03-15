import { Button, Field, Input } from '@fluentui/react-components'
import { useState } from 'react'

import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import ResultPane from '../components/ResultPane'
import { api } from '../lib/api'

export default function MouseView() {
  const [x, setX] = useState('400')
  const [y, setY] = useState('300')
  const [speed, setSpeed] = useState('1200')
  const [delay, setDelay] = useState('50')
  const [button, setButton] = useState('left')
  const [scrollDirection, setScrollDirection] = useState('up')
  const [scrollAmount, setScrollAmount] = useState('120')
  const [dragFromX, setDragFromX] = useState('400')
  const [dragFromY, setDragFromY] = useState('300')
  const [dragToX, setDragToX] = useState('600')
  const [dragToY, setDragToY] = useState('300')
  const [output, setOutput] = useState('')

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
        title="Mouse"
        description="Move the pointer, click, drag, and scroll while inspecting the exact backend responses returned by gut."
      />

      <div className="gutgd-list">
        <section className="gutgd-rowCard">
          <div className="gutgd-rowLead">
            <div className="gutgd-rowLeadGlyph">◎</div>
            <div>
              <h3>Position and move</h3>
              <p>Read the current position, jump to a point, or move along a straight path.</p>
            </div>
          </div>
          <div className="gutgd-rowBody">
            <div className="gutgd-grid2">
              <Field label="X"><Input type="number" value={x} onChange={(_, data) => setX(data.value)} /></Field>
              <Field label="Y"><Input type="number" value={y} onChange={(_, data) => setY(data.value)} /></Field>
            </div>
            <div className="gutgd-grid2">
              <Field label="Speed"><Input type="number" value={speed} onChange={(_, data) => setSpeed(data.value)} /></Field>
              <Field label="Auto delay (ms)"><Input type="number" value={delay} onChange={(_, data) => setDelay(data.value)} /></Field>
            </div>
            <div className="gutgd-rowActions">
              <Button onClick={() => run(() => api.getMousePosition())}>Read current position</Button>
              <Button appearance="primary" onClick={() => run(() => api.setMousePosition({ x: Number(x), y: Number(y), auto_delay_ms: Number(delay) }))}>Set position</Button>
              <Button onClick={() => run(() => api.moveMouseLine({ x: Number(x), y: Number(y), speed: Number(speed), auto_delay_ms: Number(delay) }))}>Move line</Button>
            </div>
          </div>
        </section>

        <section className="gutgd-rowCard">
          <div className="gutgd-rowLead">
            <div className="gutgd-rowLeadGlyph">↕</div>
            <div>
              <h3>Click, scroll, drag</h3>
              <p>Exercise click paths, four-direction scroll, and simple left-button drag motions.</p>
            </div>
          </div>
          <div className="gutgd-rowBody">
            <Field label="Button">
              <select className="gutgd-nativeSelect" value={button} onChange={(event) => setButton(event.target.value)}>
                <option value="left">left</option>
                <option value="middle">middle</option>
                <option value="right">right</option>
              </select>
            </Field>
            <div className="gutgd-rowActions">
              <Button appearance="primary" onClick={() => run(() => api.clickMouse({ button, auto_delay_ms: Number(delay) }))}>Click</Button>
              <Button onClick={() => run(() => api.doubleClickMouse({ button, auto_delay_ms: Number(delay) }))}>Double click</Button>
            </div>
            <div className="gutgd-grid2">
              <Field label="Scroll direction">
                <select className="gutgd-nativeSelect" value={scrollDirection} onChange={(event) => setScrollDirection(event.target.value)}>
                  <option value="up">up</option>
                  <option value="down">down</option>
                  <option value="left">left</option>
                  <option value="right">right</option>
                </select>
              </Field>
              <Field label="Scroll amount">
                <Input type="number" value={scrollAmount} onChange={(_, data) => setScrollAmount(data.value)} />
              </Field>
            </div>
            <div className="gutgd-rowActions">
              <Button onClick={() => run(() => api.scrollMouse({ direction: scrollDirection, amount: Number(scrollAmount), auto_delay_ms: Number(delay) }))}>Scroll</Button>
            </div>
            <div className="gutgd-grid4">
              <Field label="From X"><Input type="number" value={dragFromX} onChange={(_, data) => setDragFromX(data.value)} /></Field>
              <Field label="From Y"><Input type="number" value={dragFromY} onChange={(_, data) => setDragFromY(data.value)} /></Field>
              <Field label="To X"><Input type="number" value={dragToX} onChange={(_, data) => setDragToX(data.value)} /></Field>
              <Field label="To Y"><Input type="number" value={dragToY} onChange={(_, data) => setDragToY(data.value)} /></Field>
            </div>
            <div className="gutgd-rowActions">
              <Button onClick={() => run(() => api.dragMouse({
                from_x: Number(dragFromX),
                from_y: Number(dragFromY),
                to_x: Number(dragToX),
                to_y: Number(dragToY),
                speed: Number(speed),
                auto_delay_ms: Number(delay)
              }))}>
                Drag left button
              </Button>
            </div>
          </div>
        </section>

        <Panel title="Mouse output" description="Latest pointer response from the backend.">
          <ResultPane value={output} />
        </Panel>
      </div>
    </>
  )
}
