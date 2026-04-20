import { Button, Checkbox, Field, Input, Textarea } from '@fluentui/react-components'

import Panel from './Panel'

export default function AgentSettingsPanel({
  apiKey,
  baseURL,
  model,
  models,
  reasoningEffort,
  systemPrompt,
  strictBackgroundOnly,
  apiKeySource,
  status,
  error,
  loadingSettings,
  savingSettings,
  onAPIKeyChange,
  onBaseURLChange,
  onModelChange,
  onReasoningEffortChange,
  onSystemPromptChange,
  onStrictBackgroundOnlyChange,
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
        <Field label="Upstream provider URL">
          <Input
            placeholder="https://api.openai.com/v1"
            value={baseURL}
            onChange={(_, data) => onBaseURLChange(data.value)}
          />
        </Field>
      </div>
      <div className="gutgd-grid2">
        <Field label="Model">
          <>
            <Input
              list="gutgd-agent-model-options"
              placeholder="gpt-5.4"
              value={model}
              onChange={(_, data) => onModelChange(data.value)}
            />
            <datalist id="gutgd-agent-model-options">
              {models.map((item) => (
                <option key={item.id} value={item.id} />
              ))}
            </datalist>
          </>
        </Field>
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
      <Checkbox
        checked={strictBackgroundOnly}
        label="Strict background-only mouse mode"
        onChange={(_, data) => onStrictBackgroundOnlyChange(Boolean(data.checked))}
      />
      <div className="gutgd-agentFeedback">
        <div className="gutgd-agentMetaChips">
          <span className="gutgd-agentChip">
            <strong>Credentials</strong>
            <span>{agentCredentialSourceLabel(apiKeySource)}</span>
          </span>
          <span className="gutgd-agentChip">
            <strong>Model</strong>
            <span>{model}</span>
          </span>
          <span className="gutgd-agentChip">
            <strong>Reasoning</strong>
            <span>{reasoningEffort}</span>
          </span>
          <span className="gutgd-agentChip">
            <strong>Pointer mode</strong>
            <span>{strictBackgroundOnly ? 'Strict background-only' : 'Standard'}</span>
          </span>
          <span className="gutgd-agentChip">
            <strong>State</strong>
            <span>{loadingSettings ? 'Loading' : 'Ready'}</span>
          </span>
        </div>
        {strictBackgroundOnly ? (
          <div className="gutgd-statusNote gutgd-agentNotice">
            Raw pointer tools are disabled for the desktop agent. Cached element actions must stay on the strict background AX path and will fail instead of falling back to focused cursor input.
          </div>
        ) : null}
        {apiKeySource === 'environment' ? (
          <div className="gutgd-statusNote gutgd-agentNotice">
            Live agent runs are currently using <code>OPENAI_API_KEY</code> from the environment. The password field stays empty until you explicitly save a local key for this desktop profile.
          </div>
        ) : null}
        {status ? <div className="gutgd-statusNote gutgd-agentNotice">{status}</div> : null}
        {error ? <div className="gutgd-errorNote gutgd-agentNotice gutgd-agentErrorBlock">{error}</div> : null}
      </div>
    </Panel>
  )
}

function agentCredentialSourceLabel(value) {
  switch (value) {
    case 'saved':
      return 'Saved locally'
    case 'environment':
      return 'OPENAI_API_KEY env'
    default:
      return 'Missing'
  }
}
