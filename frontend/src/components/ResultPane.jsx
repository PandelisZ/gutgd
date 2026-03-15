import { Text } from '@fluentui/react-components'

import { formatPayload } from '../lib/format'

export default function ResultPane({ title, description, value }) {
  return (
    <div>
      {(title || description) ? (
        <div className="gutgd-cardHeader">
          <div>
            {title ? <Text as="h3" size={500} weight="semibold">{title}</Text> : null}
            {description ? <p>{description}</p> : null}
          </div>
        </div>
      ) : null}
      <pre className="gutgd-output">{formatPayload(value)}</pre>
    </div>
  )
}
