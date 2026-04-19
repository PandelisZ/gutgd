import { Button, Field, Input } from '@fluentui/react-components'
import { useState } from 'react'

import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import ResultPane from '../components/ResultPane'
import { api } from '../lib/api'

export default function WindowsView() {
  const [windows, setWindows] = useState([])
  const [handle, setHandle] = useState('0')
  const [x, setX] = useState('80')
  const [y, setY] = useState('80')
  const [width, setWidth] = useState('900')
  const [height, setHeight] = useState('700')
  const [snapshotID, setSnapshotID] = useState('')
  const [elementID, setElementID] = useState('')
  const [elementAction, setElementAction] = useState('click')
  const [output, setOutput] = useState('')

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

  async function loadWindows() {
    const result = await run(() => api.listWindows())
    setWindows(result)
  }

  async function captureAccessibilitySnapshot() {
    const result = await run(() => api.getWindowAccessibilitySnapshot({ handle: Number(handle) }))
    setSnapshotID(result.snapshot_id || '')
    const firstActionable = (result.elements || []).find((item) => (item.available_actions || []).includes(elementAction))
      || (result.elements || []).find((item) => (item.available_actions || []).length)
    if (firstActionable) {
      setElementID(firstActionable.id || '')
      if (!(firstActionable.available_actions || []).includes(elementAction) && firstActionable.available_actions?.length) {
        setElementAction(firstActionable.available_actions[0])
      }
    }
    return result
  }

  return (
    <>
      <PageHeader
        title="Windows"
        description="Enumerate windows, inspect regions, and run supported focus, move, and resize actions."
        actions={
          <Button appearance="primary" onClick={loadWindows}>Refresh windows</Button>
        }
      />

      <div className="gutgd-list">
        <section className="gutgd-rowCard">
          <div className="gutgd-rowLead">
            <div className="gutgd-rowLeadGlyph">❐</div>
            <div>
              <h3>Window list</h3>
              <p>Enumerate available windows and select one to drive supported actions.</p>
            </div>
          </div>
          <div className="gutgd-rowBody">
            <div className="gutgd-rowActions">
              <Button appearance="primary" onClick={loadWindows}>List windows</Button>
              <Button onClick={async () => {
                const active = await run(() => api.getActiveWindow())
                if (active?.handle) {
                  setHandle(String(active.handle))
                }
              }}>
                Get active window
              </Button>
            </div>
            <div className="gutgd-windowList">
              {windows.map((item) => (
                <button
                  key={`${item.handle}-${item.title}`}
                  type="button"
                  className="gutgd-windowItem"
                  onClick={() => {
                    setHandle(String(item.handle))
                    setX(String(item.region.left))
                    setY(String(item.region.top))
                    setWidth(String(item.region.width))
                    setHeight(String(item.region.height))
                    setSnapshotID('')
                    setElementID('')
                    setOutput(item)
                  }}
                >
                  <strong>{item.title || '(untitled window)'}</strong>
                  <span>Handle: {item.handle}</span>
                  <span>Region: {item.region.left}, {item.region.top}, {item.region.width}, {item.region.height}</span>
                </button>
              ))}
              {!windows.length ? <div className="gutgd-empty">No windows loaded yet.</div> : null}
            </div>
          </div>
        </section>

        <section className="gutgd-rowCard">
          <div className="gutgd-rowLead">
            <div className="gutgd-rowLeadGlyph">↔</div>
            <div>
              <h3>Selected window actions</h3>
              <p>Focus, move, resize, and probe unsupported operations with explicit feedback.</p>
            </div>
          </div>
          <div className="gutgd-rowBody">
            <Field label="Handle"><Input type="number" value={handle} onChange={(_, data) => setHandle(data.value)} /></Field>
            <div className="gutgd-rowActions">
              <Button appearance="primary" onClick={() => run(() => api.focusWindow({ handle: Number(handle) }))}>Focus</Button>
              <Button onClick={() => run(() => api.minimizeWindow({ handle: Number(handle) }))}>Minimize</Button>
              <Button onClick={() => run(() => api.restoreWindow({ handle: Number(handle) }))}>Restore</Button>
            </div>
            <div className="gutgd-grid2">
              <Field label="X"><Input type="number" value={x} onChange={(_, data) => setX(data.value)} /></Field>
              <Field label="Y"><Input type="number" value={y} onChange={(_, data) => setY(data.value)} /></Field>
            </div>
            <div className="gutgd-rowActions">
              <Button onClick={() => run(() => api.moveWindow({ handle: Number(handle), x: Number(x), y: Number(y) }))}>Move</Button>
            </div>
            <div className="gutgd-grid2">
              <Field label="Width"><Input type="number" value={width} onChange={(_, data) => setWidth(data.value)} /></Field>
              <Field label="Height"><Input type="number" value={height} onChange={(_, data) => setHeight(data.value)} /></Field>
            </div>
            <div className="gutgd-rowActions">
              <Button onClick={() => run(() => api.resizeWindow({ handle: Number(handle), width: Number(width), height: Number(height) }))}>Resize</Button>
            </div>
          </div>
        </section>

        <section className="gutgd-rowCard">
          <div className="gutgd-rowLead">
            <div className="gutgd-rowLeadGlyph">⌘</div>
            <div>
              <h3>Background AX probe</h3>
              <p>Capture a handle-targeted accessibility snapshot, then run one cached action and inspect whether it stayed in background AX mode or fell back to foreground raw input.</p>
            </div>
          </div>
          <div className="gutgd-rowBody">
            <div className="gutgd-rowActions">
              <Button appearance="primary" onClick={captureAccessibilitySnapshot}>Capture AX snapshot</Button>
              <Button
                onClick={() => run(() => api.actOnWindowAccessibilityElement({
                  snapshot_id: snapshotID,
                  element_id: elementID,
                  action: elementAction
                }))}
                disabled={!snapshotID || !elementID}
              >
                Run cached action
              </Button>
            </div>
            <Field label="Snapshot ID">
              <Input value={snapshotID} onChange={(_, data) => setSnapshotID(data.value)} />
            </Field>
            <Field label="Element ID">
              <Input value={elementID} onChange={(_, data) => setElementID(data.value)} />
            </Field>
            <Field label="Action">
              <select className="gutgd-nativeSelect" value={elementAction} onChange={(event) => setElementAction(event.target.value)}>
                <option value="click">click</option>
                <option value="focus">focus</option>
                <option value="double_click">double_click</option>
                <option value="right_click">right_click</option>
                <option value="show_menu">show_menu</option>
              </select>
            </Field>
          </div>
        </section>

        <Panel title="Window output" description="Latest window result or selected item details.">
          <ResultPane value={output} />
        </Panel>
      </div>
    </>
  )
}
