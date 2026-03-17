import { Button, Field, Input, Textarea } from '@fluentui/react-components'

import Panel from './Panel'

export default function AgentSettingsPanel({
  apiKey,
  model,
  models,
  reasoningEffort,
  systemPrompt,
  status,
  error,
  loadingSettings,
  savingSettings,
  onAPIKeyChange,
  onModelChange,
  onReasoningEffortChange,
  onSystemPromptChange,
  onSave
}) {
  return (
    <Panel
      className="gutgd-agentSettingsCard"
      title="Agent settings"
      description="These settings are stored locally for this desktop app profile."
      actions={
        <Button appearance="primary" onClick={onSave} disabled={savingSettings || loadingSettings}>
          {savingSettings ? 'Saving…' : 'Save settings'}
        </Button>
      }
    >
      <div className="gutgd-grid2">
        <Field label="API key">
          <Input type="password" value={apiKey} onChange={(_, data) => onAPIKeyChange(data.value)} />
        </Field>
        <Field label="Model">
          <select className="gutgd-nativeSelect" value={model} onChange={(event) => onModelChange(event.target.value)}>
            {models.map((item) => (
              <option key={item.id} value={item.id}>{item.id}</option>
            ))}
            {!models.some((item) => item.id === model) ? <option value={model}>{model}</option> : null}
          </select>
        </Field>
      </div>
      <div className="gutgd-grid2">
        <Field label="Reasoning effort">
          <select className="gutgd-nativeSelect" value={reasoningEffort} onChange={(event) => onReasoningEffortChange(event.target.value)}>
            <option value="none">none</option>
            <option value="minimal">minimal</option>
            <option value="low">low</option>
            <option value="medium">medium</option>
            <option value="high">high</option>
            <option value="xhigh">xhigh</option>
          </select>
        </Field>
        <Field label="System prompt">
          <Textarea value={systemPrompt} onChange={(_, data) => onSystemPromptChange(data.value)} />
        </Field>
      </div>
      <div className="gutgd-agentFeedback">
        <div className="gutgd-agentMetaChips">
          <span className="gutgd-agentChip">
            <strong>Model</strong>
            <span>{model}</span>
          </span>
          <span className="gutgd-agentChip">
            <strong>Reasoning</strong>
            <span>{reasoningEffort}</span>
          </span>
          <span className="gutgd-agentChip">
            <strong>State</strong>
            <span>{loadingSettings ? 'Loading' : 'Ready'}</span>
          </span>
        </div>
        {status ? <div className="gutgd-statusNote gutgd-agentNotice">{status}</div> : null}
        {error ? <div className="gutgd-errorNote gutgd-agentNotice gutgd-agentErrorBlock">{error}</div> : null}
      </div>
    </Panel>
  )
}
