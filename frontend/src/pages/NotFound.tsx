import { Link } from 'react-router-dom'
import { Card } from '../components/Card'

export function NotFound() {
  return (
    <div>
      <div className="page-head">
        <h1 className="page-title">Page not found</h1>
        <p className="page-subtitle">This address does not match any page in the dashboard.</p>
      </div>
      <Card>
        <p className="note" style={{ marginTop: 0 }}>
          Go to the <Link to="/">dashboard</Link> or the <Link to="/findings">findings list</Link>.
        </p>
      </Card>
    </div>
  )
}
