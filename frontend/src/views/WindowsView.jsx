import { Button, Field, Input } from '@fluentui/react-components'
import { useState } from 'react'

import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import ResultPane from '../components/ResultPane'
import { api } from '../lib/api'

const strictBackgroundActions = ['click', 'double_click', 'focus', 'right_click', 'show_menu']
const virtualStageMaxWidth = 560

export default function WindowsView() {
  const [windows, setWindows] = useState([])
  const [handle, setHandle] = useState('0')
  const [x, setX] = useState('80')
  const [y, setY] = useState('80')
  const [width, setWidth] = useState('900')
  const [height, setHeight] = useState('700')
  const [snapshot, setSnapshot] = useState(null)
  const [snapshotID, setSnapshotID] = useState('')
  const [elementID, setElementID] = useState('')
  const [elementAction, setElementAction] = useState('click')
  const [virtualResult, setVirtualResult] = useState(null)
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
    setSnapshot(result)
    setSnapshotID(result.snapshot_id || '')
    setVirtualResult(null)

    const firstActionable = (result.elements || []).find((item) => (item.background_safe_actions || []).length)
      || (result.elements || []).find((item) => (item.available_actions || []).length)
      || null
    if (firstActionable) {
      setElementID(firstActionable.id || '')
      setElementAction(preferredAction(firstActionable, elementAction))
    } else {
      setElementID('')
      setElementAction('click')
    }
    return result
  }

  async function resolveVirtualPoint(point) {
    if (!snapshotID) {
      return
    }
    const result = await run(() => api.resolveBackgroundWindowPoint({
      snapshot_id: snapshotID,
      x: point.x,
      y: point.y
    }))
    setVirtualResult(result)
    if (result.element_id) {
      setElementID(result.element_id)
      setElementAction(preferredAction(result.element, elementAction))
    }
    return result
  }

  async function performVirtualAction() {
    if (!snapshotID) {
      return
    }
    const payload = {
      snapshot_id: snapshotID,
      action: elementAction
    }
    if (elementID) {
      payload.element_id = elementID
    } else if (virtualResult?.requested_point) {
      payload.point = virtualResult.requested_point
    }

    const result = await run(() => api.performBackgroundWindowAction(payload))
    setVirtualResult((previous) => ({
      ...(previous || {}),
      requested_point: result.requested_point || previous?.requested_point || null,
      screen_point: result.screen_point,
      snapped: result.snapped,
      element_id: result.element_id,
      mode: result.mode
    }))
    if (result.element_id) {
      setElementID(result.element_id)
      setElementAction(preferredAction(result.element, elementAction))
    }
    return result
  }

  function handleStageClick(event) {
    const metrics = buildStageMetrics(snapshot?.window)
    if (!snapshot?.window || metrics.scale <= 0) {
      return
    }
    const rect = event.currentTarget.getBoundingClientRect()
    const relativeX = clamp(Math.round((event.clientX - rect.left) / metrics.scale), 0, Math.max(0, snapshot.window.region.width - 1))
    const relativeY = clamp(Math.round((event.clientY - rect.top) / metrics.scale), 0, Math.max(0, snapshot.window.region.height - 1))
    void resolveVirtualPoint({ x: relativeX, y: relativeY })
  }

  const selectedElement = (snapshot?.elements || []).find((item) => item.id === elementID) || null
  const stageMetrics = buildStageMetrics(snapshot?.window)
  const selectableActions = selectedElement?.background_safe_actions?.length
    ? selectedElement.background_safe_actions
    : strictBackgroundActions
  const canRunVirtualAction = Boolean(snapshotID && (elementID || virtualResult?.requested_point) && selectableActions.includes(elementAction))

  return (
    <>
      <PageHeader
        title="Windows"
        description="Enumerate windows, inspect regions, and run supported focus, move, resize, and strict background virtual-mouse actions."
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
                    setSnapshot(null)
                    setSnapshotID('')
                    setElementID('')
                    setVirtualResult(null)
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
              <h3>Background virtual mouse</h3>
              <p>Capture a background window snapshot, click inside the scaled stage to resolve a strict AX-backed target, then run a virtual action without focusing the window or moving the real cursor.</p>
            </div>
          </div>
          <div className="gutgd-rowBody">
            <div className="gutgd-rowActions">
              <Button appearance="primary" onClick={captureAccessibilitySnapshot}>Capture AX snapshot</Button>
              <Button onClick={performVirtualAction} disabled={!canRunVirtualAction}>Run strict background action</Button>
            </div>

            <div className="gutgd-grid2">
              <Field label="Snapshot ID">
                <Input value={snapshotID} onChange={(_, data) => setSnapshotID(data.value)} />
              </Field>
              <Field label="Element ID">
                <Input value={elementID} onChange={(_, data) => setElementID(data.value)} />
              </Field>
            </div>

            <Field label="Action">
              <select className="gutgd-nativeSelect" value={elementAction} onChange={(event) => setElementAction(event.target.value)}>
                {selectableActions.map((action) => (
                  <option key={action} value={action}>{action}</option>
                ))}
              </select>
            </Field>

            <div className="gutgd-chipRow">
              {(selectedElement?.background_safe_actions || []).map((action) => (
                <span key={action} className="gutgd-chip">{action}</span>
              ))}
              {!selectedElement?.background_safe_actions?.length ? <span className="gutgd-chip gutgd-chip-muted">No strict background actions resolved yet</span> : null}
            </div>

            {snapshot?.window ? (
              <div className="gutgd-virtualStageWrap">
                <div className="gutgd-virtualStageMeta">
                  <strong>{snapshot.window.title || '(untitled window)'}</strong>
                  <span>{snapshot.window.region.width}×{snapshot.window.region.height} window space</span>
                  <span>{virtualResult?.snapped ? 'Snapped to actionable point' : 'Direct point resolution'}</span>
                </div>
                <button
                  type="button"
                  className="gutgd-virtualStage"
                  style={{ width: `${stageMetrics.width}px`, height: `${stageMetrics.height}px` }}
                  onClick={handleStageClick}
                >
                  {(snapshot.elements || []).map((item) => {
                    const style = stageElementStyle(item, snapshot.window.region, stageMetrics.scale)
                    if (!style) {
                      return null
                    }
                    const strict = (item.background_safe_actions || []).length > 0
                    return (
                      <div
                        key={item.id}
                        className={[
                          'gutgd-virtualStageElement',
                          strict ? 'gutgd-virtualStageElement-actionable' : '',
                          item.id === elementID ? 'gutgd-virtualStageElement-selected' : ''
                        ].filter(Boolean).join(' ')}
                        style={style}
                      />
                    )
                  })}
                  {virtualCursorStyle(virtualResult, snapshot.window.region, stageMetrics.scale) ? (
                    <div className="gutgd-virtualCursor" style={virtualCursorStyle(virtualResult, snapshot.window.region, stageMetrics.scale)} />
                  ) : null}
                </button>
                <p className="gutgd-stageHint">
                  Click anywhere in the stage to resolve a strict background target. The pink cursor is virtual only; the real desktop pointer stays untouched.
                </p>
              </div>
            ) : (
              <div className="gutgd-empty">Capture a snapshot to start the virtual background workflow.</div>
            )}
          </div>
        </section>

        <Panel title="Window output" description="Latest window result, strict background resolve, or action details.">
          <ResultPane value={output} />
        </Panel>
      </div>
    </>
  )
}

function preferredAction(element, current) {
  const actions = element?.background_safe_actions || []
  if (!actions.length) {
    return current
  }
  if (actions.includes(current)) {
    return current
  }
  return actions[0]
}

function buildStageMetrics(window) {
  const width = window?.region?.width || 0
  const height = window?.region?.height || 0
  if (!width || !height) {
    return { width: 0, height: 0, scale: 0 }
  }
  const stageWidth = Math.min(virtualStageMaxWidth, width)
  const scale = stageWidth / width
  return {
    width: Math.round(stageWidth),
    height: Math.round(height * scale),
    scale
  }
}

function stageElementStyle(element, windowRegion, scale) {
  if (!element?.screen_region || !windowRegion || scale <= 0) {
    return null
  }
  return {
    left: `${(element.screen_region.left - windowRegion.left) * scale}px`,
    top: `${(element.screen_region.top - windowRegion.top) * scale}px`,
    width: `${Math.max(1, element.screen_region.width * scale)}px`,
    height: `${Math.max(1, element.screen_region.height * scale)}px`
  }
}

function virtualCursorStyle(virtualResult, windowRegion, scale) {
  if (!virtualResult?.screen_point || !windowRegion || scale <= 0) {
    return null
  }
  return {
    left: `${(virtualResult.screen_point.x - windowRegion.left) * scale}px`,
    top: `${(virtualResult.screen_point.y - windowRegion.top) * scale}px`
  }
}

function clamp(value, min, max) {
  return Math.max(min, Math.min(max, value))
}
