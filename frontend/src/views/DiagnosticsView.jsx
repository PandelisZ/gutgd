import { Badge, Button, Switch, Text } from '@fluentui/react-components'
import { useEffect, useMemo, useState } from 'react'

import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import ResultPane from '../components/ResultPane'
import { api } from '../lib/api'

function availabilityColor(value) {
  if (value === 'available') {
    return 'success'
  }
  return 'warning'
}

export default function DiagnosticsView({ bridgeMode }) {
  const [mutable, setMutable] = useState(false)
  const [diagnostics, setDiagnostics] = useState(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const summaryRows = useMemo(() => {
    if (!diagnostics) {
      return []
    }
    const { report } = diagnostics
    return [
      ['Ready', report.ready ? 'yes' : 'no'],
      ['Platform', report.platform],
      ['Backend', `${report.backend?.name || 'unknown'} / ${report.backend?.binding_state || 'unknown'}`],
      ['Artifacts', diagnostics.artifacts_path],
      ['Working directory', diagnostics.working_dir],
      ['Runtime', diagnostics.runtime]
    ]
  }, [diagnostics])

  async function loadDiagnostics(nextMutable = mutable) {
    setLoading(true)
    setError('')
    try {
      const result = await api.getDiagnostics(nextMutable)
      setDiagnostics(result)
    } catch (nextError) {
      setError(nextError.message || String(nextError))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadDiagnostics(mutable)
  }, [mutable])

  return (
    <>
      <PageHeader
        title="Diagnostics"
        description="Inspect capability readiness, backend notes, and unsupported feature areas before driving the live desktop APIs."
        actions={
          <Button appearance="primary" onClick={() => loadDiagnostics()}>
            Refresh diagnostics
          </Button>
        }
      />

      <section className="gutgd-hero">
        <div className="gutgd-card gutgd-heroPrimary">
          <div className="gutgd-heroGlyph">⌘</div>
          <div>
            <Text as="h2" size={700} weight="semibold">Environment report</Text>
            <Text as="p">Readiness, backend details, and capability gates for the current host session.</Text>
          </div>
        </div>

        <div className="gutgd-heroStats">
          <div className="gutgd-statCard">
            <span className="gutgd-statLabel">Bridge</span>
            <Badge color={bridgeMode === 'desktop' ? 'success' : bridgeMode === 'preview' ? 'warning' : 'danger'}>
              {bridgeMode === 'desktop' ? 'Connected' : bridgeMode === 'preview' ? 'Preview fallback' : 'Unavailable'}
            </Badge>
          </div>
          <div className="gutgd-statCard">
            <span className="gutgd-statLabel">Ready</span>
            <Text weight="semibold">{diagnostics?.report?.ready ? 'Ready' : 'Attention needed'}</Text>
          </div>
        </div>
      </section>

      <div className="gutgd-list">
        <Panel
          title="Environment report"
          description="Readiness, platform, backend, and evaluation gates."
          actions={
            <Switch
              checked={mutable}
              label="Request mutable evaluation"
              onChange={(_, data) => {
                setMutable(Boolean(data.checked))
              }}
            />
          }
        >
          {error ? <div className="gutgd-empty">{error}</div> : null}
          <div className="gutgd-summaryGrid">
            {summaryRows.map(([label, value]) => (
              <div key={label} className="gutgd-summaryRow">
                <span className="gutgd-summaryRowLabel">{label}</span>
                <strong>{value}</strong>
              </div>
            ))}
          </div>
          {diagnostics?.report?.reasons?.length ? (
            <ul className="gutgd-reasonList">
              {diagnostics.report.reasons.map((reason) => (
                <li key={reason}>{reason}</li>
              ))}
            </ul>
          ) : null}
          {!loading && !diagnostics && !error ? <div className="gutgd-empty">Waiting for diagnostics.</div> : null}
        </Panel>

        <Panel
          title="Capabilities"
          description="Supported, unavailable, and unsupported feature surfaces."
        >
          <div className="gutgd-featureList">
            {(diagnostics?.feature_status || []).map((item) => (
              <article key={item.id} className="gutgd-featureItem">
                <header>
                  <strong>{item.id}</strong>
                  <Badge color={availabilityColor(item.availability)}>{item.availability}</Badge>
                </header>
                <p>{item.reason || 'No additional detail.'}</p>
              </article>
            ))}
            {!diagnostics?.feature_status?.length && !error ? <div className="gutgd-empty">No capability data loaded yet.</div> : null}
          </div>
        </Panel>

        <Panel title="Backend notes" description="Raw diagnostics payload for debugging and issue capture.">
          <ResultPane value={error ? { error } : diagnostics} />
        </Panel>
      </div>
    </>
  )
}
