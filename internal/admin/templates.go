package admin

import "html/template"

// Parsed template variables.
var (
	DashboardTmpl    *template.Template
	NewWalletTmpl    *template.Template
	WalletDetailTmpl *template.Template
	ConfigTmpl       *template.Template
	ResetConfirmTmpl *template.Template
)

// maskAPIKey shows first 8 and last 4 characters of an API key.
func maskAPIKey(key string) string {
	if len(key) <= 12 {
		return key
	}
	return key[:8] + "..." + key[len(key)-4:]
}

// deref safely dereferences a *string pointer for template use.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func init() {
	funcMap := template.FuncMap{
		"maskAPIKey": maskAPIKey,
		"deref":      deref,
	}
	DashboardTmpl = template.Must(template.New("dashboard").Funcs(funcMap).Parse(dashboardHTML))
	NewWalletTmpl = template.Must(template.New("newWallet").Funcs(funcMap).Parse(newWalletHTML))
	WalletDetailTmpl = template.Must(template.New("walletDetail").Funcs(funcMap).Parse(walletDetailHTML))
	ConfigTmpl = template.Must(template.New("config").Funcs(funcMap).Parse(configHTML))
	ResetConfirmTmpl = template.Must(template.New("resetConfirm").Funcs(funcMap).Parse(resetConfirmHTML))
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Lipila Mock - Admin</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #333; }
        nav { background: #1a1a2e; color: white; padding: 1rem 2rem; display: flex; gap: 2rem; align-items: center; }
        nav .brand { font-weight: bold; font-size: 1.2rem; }
        .container { max-width: 1200px; margin: 1.5rem auto; padding: 0 1rem; }
        .flash { padding: 0.75rem 1rem; border-radius: 6px; margin-bottom: 1rem; font-size: 0.9rem; }
        .flash-success { background: #d4edda; color: #155724; border: 1px solid #c3e6cb; }
        .flash-error { background: #f8d7da; color: #721c24; border: 1px solid #f5c6cb; }
        .card { background: white; border-radius: 8px; padding: 1.5rem; margin-bottom: 1.5rem; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
        .card h2 { margin-bottom: 1rem; font-size: 1.1rem; color: #1a1a2e; border-bottom: 2px solid #eee; padding-bottom: 0.5rem; }
        .grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 1.5rem; }
        @media (max-width: 768px) { .grid-2 { grid-template-columns: 1fr; } }
        table { width: 100%; border-collapse: collapse; }
        th, td { padding: 0.5rem 0.75rem; text-align: left; border-bottom: 1px solid #eee; font-size: 0.85rem; }
        th { font-weight: 600; color: #666; background: #fafafa; }
        .badge { padding: 2px 8px; border-radius: 4px; font-size: 0.75rem; font-weight: 600; display: inline-block; }
        .badge-success { background: #d4edda; color: #155724; }
        .badge-pending { background: #fff3cd; color: #856404; }
        .badge-failed { background: #f8d7da; color: #721c24; }
        .badge-active { background: #d4edda; color: #155724; }
        .badge-inactive { background: #e2e3e5; color: #383d41; }
        .mono { font-family: "SFMono-Regular", Consolas, monospace; font-size: 0.8rem; }
        .table-wrap { overflow-x: auto; }
        form { margin: 0; }
        label { display: block; font-size: 0.85rem; font-weight: 500; margin-bottom: 0.25rem; color: #555; }
        input[type="text"], input[type="number"], select {
            width: 100%; padding: 0.5rem; border: 1px solid #ddd; border-radius: 4px;
            font-size: 0.85rem; margin-bottom: 0.75rem;
        }
        input[type="text"]:focus, input[type="number"]:focus, select:focus {
            outline: none; border-color: #1a1a2e;
        }
        .form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; }
        .form-row-3 { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 0.75rem; }
        .btn { padding: 0.5rem 1rem; border: none; border-radius: 4px; cursor: pointer; font-size: 0.85rem; font-weight: 500; }
        .btn-primary { background: #1a1a2e; color: white; }
        .btn-primary:hover { background: #16213e; }
        .btn-danger { background: #dc3545; color: white; }
        .btn-danger:hover { background: #c82333; }
        .btn-sm { padding: 0.35rem 0.75rem; font-size: 0.8rem; }
        .actions { display: flex; gap: 0.5rem; align-items: center; margin-top: 1rem; }
        .checkbox-row { display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.75rem; }
        .checkbox-row input[type="checkbox"] { width: auto; margin: 0; }
        .text-muted { color: #888; font-size: 0.8rem; }
        .overflow-wrap { word-break: break-all; }
    </style>
</head>
<body>
    <nav>
        <span class="brand">Lipila Mock Admin</span>
    </nav>
    <div class="container">
        {{if .Flash}}<div class="flash flash-success">{{.Flash}}</div>{{end}}
        {{if .Error}}<div class="flash flash-error">{{.Error}}</div>{{end}}

        <!-- Wallets Section -->
        <div class="card">
            <h2>Wallets</h2>
            {{if .Wallets}}
            <div class="table-wrap">
            <table>
                <thead>
                    <tr>
                        <th>Name</th>
                        <th>API Key</th>
                        <th>Till Number</th>
                        <th>Balance</th>
                        <th>Status</th>
                        <th>Created</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Wallets}}
                    <tr>
                        <td><a href="/admin/wallets/{{.ID}}">{{.Name}}</a></td>
                        <td class="mono" title="{{.APIKey}}">{{maskAPIKey .APIKey}}</td>
                        <td class="mono">{{.TillNumber}}</td>
                        <td>{{.Currency}} {{printf "%.2f" .Balance}}</td>
                        <td>
                            {{if .IsActive}}<span class="badge badge-active">Active</span>
                            {{else}}<span class="badge badge-inactive">Inactive</span>{{end}}
                        </td>
                        <td>{{.CreatedAt.Format "2006-01-02 15:04"}}</td>
                        <td>
                            <form method="POST" action="/admin/wallets/{{.ID}}/toggle" style="display:inline">
                                <button type="submit" class="btn btn-sm {{if .IsActive}}btn-danger{{else}}btn-primary{{end}}">
                                    {{if .IsActive}}Deactivate{{else}}Activate{{end}}
                                </button>
                            </form>
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
            </div>
            {{else}}
            <p class="text-muted">No wallets yet.</p>
            {{end}}
        </div>

        <div class="grid-2">
            <!-- Create Wallet Form -->
            <div class="card">
                <h2>Create New Wallet</h2>
                <form method="POST" action="/admin/create-wallet">
                    <label for="name">Wallet Name</label>
                    <input type="text" id="name" name="name" placeholder="My Merchant" required>
                    <div class="form-row">
                        <div>
                            <label for="balance">Initial Balance</label>
                            <input type="number" id="balance" name="balance" value="10000" step="0.01" min="0" required>
                        </div>
                        <div>
                            <label for="currency">Currency</label>
                            <select id="currency" name="currency">
                                <option value="ZMW">ZMW</option>
                                <option value="USD">USD</option>
                            </select>
                        </div>
                    </div>
                    <button type="submit" class="btn btn-primary">Create Wallet</button>
                </form>
            </div>

            <!-- Reset Database -->
            <div class="card">
                <h2>Database Management</h2>
                <p style="margin-bottom: 1rem; font-size: 0.85rem; color: #666;">
                    Reset the database to clear all wallets, transactions, and callback logs.
                    A new default wallet will be created automatically.
                </p>
                <a href="/admin/reset" class="btn btn-danger" style="text-decoration:none; display:inline-block;">Reset Entire Database</a>
            </div>
        </div>

        <!-- Test Scenarios -->
        <div class="card">
            <h2>Test Scenarios</h2>
            <p style="margin-bottom: 1rem; font-size: 0.85rem; color: #666;">
                Seed test data into the first wallet for development and testing.
            </p>
            <div style="display: flex; gap: 0.5rem; flex-wrap: wrap;">
                <form method="POST" action="/admin/seed-test-data" style="display:inline">
                    <button type="submit" class="btn btn-primary">Seed Test Data</button>
                </form>
                <form method="POST" action="/admin/seed-stuck-pending" style="display:inline">
                    <button type="submit" class="btn btn-primary">Simulate Stuck Pending</button>
                </form>
                <form method="POST" action="/admin/seed-random-history" style="display:inline">
                    <button type="submit" class="btn btn-primary">Generate 30-Day History</button>
                </form>
            </div>
            <div style="margin-top: 0.75rem;">
                <p class="text-muted"><strong>Seed Test Data</strong> — Creates a low-balance wallet + ~22 transactions covering every status, payment type, and error message variant.</p>
                <p class="text-muted"><strong>Simulate Stuck Pending</strong> — Creates 7 pending transactions aged 6h to 96h old.</p>
                <p class="text-muted"><strong>Generate 30-Day History</strong> — Creates 50 random transactions spread over the last 30 days with realistic status distribution (70% success, 20% failed, 10% pending).</p>
            </div>
        </div>

        <!-- Simulation Config -->
        <div class="card">
            <h2>Simulation Config</h2>
            <form method="POST" action="/admin/update-config">
                <div class="form-row-3">
                    <div>
                        <label for="mtn_success_rate">MTN Success Rate (%)</label>
                        <input type="number" id="mtn_success_rate" name="mtn_success_rate" value="{{.Config.MtnSuccessRate}}" min="0" max="100">
                    </div>
                    <div>
                        <label for="airtel_success_rate">Airtel Success Rate (%)</label>
                        <input type="number" id="airtel_success_rate" name="airtel_success_rate" value="{{.Config.AirtelSuccessRate}}" min="0" max="100">
                    </div>
                    <div>
                        <label for="zamtel_success_rate">Zamtel Success Rate (%)</label>
                        <input type="number" id="zamtel_success_rate" name="zamtel_success_rate" value="{{.Config.ZamtelSuccessRate}}" min="0" max="100">
                    </div>
                </div>
                <div class="form-row-3">
                    <div>
                        <label for="card_success_rate">Card Success Rate (%)</label>
                        <input type="number" id="card_success_rate" name="card_success_rate" value="{{.Config.CardSuccessRate}}" min="0" max="100">
                    </div>
                    <div>
                        <label for="bank_success_rate">Bank Success Rate (%)</label>
                        <input type="number" id="bank_success_rate" name="bank_success_rate" value="{{.Config.BankSuccessRate}}" min="0" max="100">
                    </div>
                    <div>
                        <label for="processing_delay_seconds">Processing Delay (seconds)</label>
                        <input type="number" id="processing_delay_seconds" name="processing_delay_seconds" value="{{.Config.ProcessingDelaySeconds}}" min="0" max="300">
                    </div>
                </div>
                <div class="form-row">
                    <div class="checkbox-row">
                        <input type="checkbox" id="enable_random_timeouts" name="enable_random_timeouts" {{if .Config.EnableRandomTimeouts}}checked{{end}}>
                        <label for="enable_random_timeouts" style="margin-bottom:0">Enable Random Timeouts</label>
                    </div>
                    <div>
                        <label for="timeout_probability">Timeout Probability (%)</label>
                        <input type="number" id="timeout_probability" name="timeout_probability" value="{{.Config.TimeoutProbability}}" min="0" max="100">
                    </div>
                </div>
                <button type="submit" class="btn btn-primary">Save Config</button>
            </form>
        </div>

        <!-- Recent Transactions -->
        <div class="card">
            <h2>Recent Transactions (Last 20)</h2>
            {{if .Transactions}}
            <div class="table-wrap">
            <table>
                <thead>
                    <tr>
                        <th>Identifier</th>
                        <th>Reference</th>
                        <th>Type</th>
                        <th>Payment</th>
                        <th>Amount</th>
                        <th>Status</th>
                        <th>Account</th>
                        <th>Message</th>
                        <th>Created</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Transactions}}
                    <tr>
                        <td class="mono" style="font-size:0.75rem">{{.Identifier}}</td>
                        <td class="mono" style="font-size:0.75rem">{{.ReferenceID}}</td>
                        <td>{{.Type}}</td>
                        <td>{{.PaymentType}}</td>
                        <td style="white-space:nowrap">{{.Currency}} {{printf "%.2f" .Amount}}</td>
                        <td>
                            <span class="badge {{if eq .Status "Successful"}}badge-success{{else if eq .Status "Pending"}}badge-pending{{else}}badge-failed{{end}}">
                                {{.Status}}
                            </span>
                        </td>
                        <td class="mono" style="font-size:0.75rem">{{.AccountNumber}}</td>
                        <td style="font-size:0.8rem; max-width:200px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" title="{{if .Message}}{{deref .Message}}{{end}}">
                            {{if .Message}}{{deref .Message}}{{else}}-{{end}}
                        </td>
                        <td style="white-space:nowrap">{{.CreatedAt.Format "01-02 15:04:05"}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
            </div>
            {{else}}
            <p class="text-muted">No transactions yet. Send a request to the API to get started.</p>
            {{end}}
        </div>
    </div>
</body>
</html>`

const newWalletHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>New Wallet - Lipila Mock Admin</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #333; }
        nav { background: #1a1a2e; color: white; padding: 1rem 2rem; display: flex; gap: 2rem; align-items: center; }
        nav .brand { font-weight: bold; font-size: 1.2rem; }
        nav a { color: #ccc; text-decoration: none; font-size: 0.9rem; }
        nav a:hover { color: white; }
        .container { max-width: 600px; margin: 1.5rem auto; padding: 0 1rem; }
        .flash-error { padding: 0.75rem 1rem; border-radius: 6px; margin-bottom: 1rem; font-size: 0.9rem; background: #f8d7da; color: #721c24; border: 1px solid #f5c6cb; }
        .card { background: white; border-radius: 8px; padding: 1.5rem; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
        .card h2 { margin-bottom: 1rem; font-size: 1.1rem; color: #1a1a2e; border-bottom: 2px solid #eee; padding-bottom: 0.5rem; }
        label { display: block; font-size: 0.85rem; font-weight: 500; margin-bottom: 0.25rem; color: #555; }
        input[type="text"], input[type="number"], select {
            width: 100%; padding: 0.5rem; border: 1px solid #ddd; border-radius: 4px;
            font-size: 0.85rem; margin-bottom: 0.75rem;
        }
        input[type="text"]:focus, input[type="number"]:focus, select:focus { outline: none; border-color: #1a1a2e; }
        .form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; }
        .btn { padding: 0.5rem 1rem; border: none; border-radius: 4px; cursor: pointer; font-size: 0.85rem; font-weight: 500; }
        .btn-primary { background: #1a1a2e; color: white; }
        .btn-primary:hover { background: #16213e; }
        .text-muted { color: #888; font-size: 0.8rem; margin-bottom: 1rem; }
    </style>
</head>
<body>
    <nav>
        <span class="brand">Lipila Mock Admin</span>
        <a href="/admin/">Dashboard</a>
    </nav>
    <div class="container">
        {{if .Error}}<div class="flash-error">{{.Error}}</div>{{end}}
        <div class="card">
            <h2>Create New Wallet</h2>
            <p class="text-muted">API key and till number will be auto-generated on submit.</p>
            <form method="POST" action="/admin/wallets">
                <label for="name">Wallet Name</label>
                <input type="text" id="name" name="name" placeholder="My Merchant" required>
                <div class="form-row">
                    <div>
                        <label for="balance">Initial Balance</label>
                        <input type="number" id="balance" name="balance" value="10000" step="0.01" min="0" required>
                    </div>
                    <div>
                        <label for="currency">Currency</label>
                        <select id="currency" name="currency">
                            <option value="ZMW">ZMW</option>
                            <option value="USD">USD</option>
                        </select>
                    </div>
                </div>
                <button type="submit" class="btn btn-primary">Create Wallet</button>
            </form>
        </div>
    </div>
</body>
</html>`

const walletDetailHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Wallet.Name}} - Lipila Mock Admin</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #333; }
        nav { background: #1a1a2e; color: white; padding: 1rem 2rem; display: flex; gap: 2rem; align-items: center; }
        nav .brand { font-weight: bold; font-size: 1.2rem; }
        nav a { color: #ccc; text-decoration: none; font-size: 0.9rem; }
        nav a:hover { color: white; }
        .container { max-width: 1200px; margin: 1.5rem auto; padding: 0 1rem; }
        .card { background: white; border-radius: 8px; padding: 1.5rem; margin-bottom: 1.5rem; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
        .card h2 { margin-bottom: 1rem; font-size: 1.1rem; color: #1a1a2e; border-bottom: 2px solid #eee; padding-bottom: 0.5rem; }
        .detail-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; }
        .detail-item label { font-size: 0.8rem; color: #888; margin-bottom: 0.15rem; display: block; }
        .detail-item .value { font-size: 0.95rem; font-weight: 500; }
        .mono { font-family: "SFMono-Regular", Consolas, monospace; font-size: 0.8rem; }
        .badge { padding: 2px 8px; border-radius: 4px; font-size: 0.75rem; font-weight: 600; display: inline-block; }
        .badge-success, .badge-active { background: #d4edda; color: #155724; }
        .badge-pending { background: #fff3cd; color: #856404; }
        .badge-failed { background: #f8d7da; color: #721c24; }
        .badge-inactive { background: #e2e3e5; color: #383d41; }
        table { width: 100%; border-collapse: collapse; }
        th, td { padding: 0.5rem 0.75rem; text-align: left; border-bottom: 1px solid #eee; font-size: 0.85rem; }
        th { font-weight: 600; color: #666; background: #fafafa; }
        .table-wrap { overflow-x: auto; }
        .text-muted { color: #888; font-size: 0.8rem; }
        .btn { padding: 0.5rem 1rem; border: none; border-radius: 4px; cursor: pointer; font-size: 0.85rem; font-weight: 500; }
        .btn-primary { background: #1a1a2e; color: white; }
        .btn-primary:hover { background: #16213e; }
        .btn-danger { background: #dc3545; color: white; }
        .btn-danger:hover { background: #c82333; }
        .btn-sm { padding: 0.35rem 0.75rem; font-size: 0.8rem; }
        .actions { display: flex; gap: 0.5rem; align-items: center; margin-top: 1rem; }
        @media (max-width: 768px) { .detail-grid { grid-template-columns: 1fr; } }
    </style>
</head>
<body>
    <nav>
        <span class="brand">Lipila Mock Admin</span>
        <a href="/admin/">Dashboard</a>
    </nav>
    <div class="container">
        <!-- Wallet Details -->
        <div class="card">
            <h2>Wallet: {{.Wallet.Name}}</h2>
            <div class="detail-grid">
                <div class="detail-item">
                    <label>ID</label>
                    <div class="value mono">{{.Wallet.ID}}</div>
                </div>
                <div class="detail-item">
                    <label>Status</label>
                    <div class="value">
                        {{if .Wallet.IsActive}}<span class="badge badge-active">Active</span>
                        {{else}}<span class="badge badge-inactive">Inactive</span>{{end}}
                    </div>
                </div>
                <div class="detail-item">
                    <label>API Key</label>
                    <div class="value mono" style="word-break:break-all">{{.Wallet.APIKey}}</div>
                </div>
                <div class="detail-item">
                    <label>Till Number</label>
                    <div class="value mono">{{.Wallet.TillNumber}}</div>
                </div>
                <div class="detail-item">
                    <label>Balance</label>
                    <div class="value">{{.Wallet.Currency}} {{printf "%.2f" .Wallet.Balance}}</div>
                </div>
                <div class="detail-item">
                    <label>Created</label>
                    <div class="value">{{.Wallet.CreatedAt.Format "2006-01-02 15:04:05"}}</div>
                </div>
            </div>
            <div class="actions">
                <form method="POST" action="/admin/wallets/{{.Wallet.ID}}/toggle">
                    <button type="submit" class="btn btn-sm {{if .Wallet.IsActive}}btn-danger{{else}}btn-primary{{end}}">
                        {{if .Wallet.IsActive}}Deactivate{{else}}Activate{{end}}
                    </button>
                </form>
            </div>
        </div>

        <!-- Wallet Transactions -->
        <div class="card">
            <h2>Transactions (Last 50)</h2>
            {{if .Transactions}}
            <div class="table-wrap">
            <table>
                <thead>
                    <tr>
                        <th>Identifier</th>
                        <th>Reference</th>
                        <th>Type</th>
                        <th>Payment</th>
                        <th>Amount</th>
                        <th>Status</th>
                        <th>Account</th>
                        <th>Message</th>
                        <th>Created</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Transactions}}
                    <tr>
                        <td class="mono" style="font-size:0.75rem">{{.Identifier}}</td>
                        <td class="mono" style="font-size:0.75rem">{{.ReferenceID}}</td>
                        <td>{{.Type}}</td>
                        <td>{{.PaymentType}}</td>
                        <td style="white-space:nowrap">{{.Currency}} {{printf "%.2f" .Amount}}</td>
                        <td>
                            <span class="badge {{if eq .Status "Successful"}}badge-success{{else if eq .Status "Pending"}}badge-pending{{else}}badge-failed{{end}}">
                                {{.Status}}
                            </span>
                        </td>
                        <td class="mono" style="font-size:0.75rem">{{.AccountNumber}}</td>
                        <td style="font-size:0.8rem; max-width:200px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" title="{{if .Message}}{{deref .Message}}{{end}}">
                            {{if .Message}}{{deref .Message}}{{else}}-{{end}}
                        </td>
                        <td style="white-space:nowrap">{{.CreatedAt.Format "01-02 15:04:05"}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
            </div>
            {{else}}
            <p class="text-muted">No transactions for this wallet yet.</p>
            {{end}}
        </div>
    </div>
</body>
</html>`

const configHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Simulation Config - Lipila Mock Admin</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #333; }
        nav { background: #1a1a2e; color: white; padding: 1rem 2rem; display: flex; gap: 2rem; align-items: center; }
        nav .brand { font-weight: bold; font-size: 1.2rem; }
        nav a { color: #ccc; text-decoration: none; font-size: 0.9rem; }
        nav a:hover { color: white; }
        .container { max-width: 800px; margin: 1.5rem auto; padding: 0 1rem; }
        .flash { padding: 0.75rem 1rem; border-radius: 6px; margin-bottom: 1rem; font-size: 0.9rem; }
        .flash-success { background: #d4edda; color: #155724; border: 1px solid #c3e6cb; }
        .flash-error { background: #f8d7da; color: #721c24; border: 1px solid #f5c6cb; }
        .card { background: white; border-radius: 8px; padding: 1.5rem; margin-bottom: 1.5rem; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
        .card h2 { margin-bottom: 1rem; font-size: 1.1rem; color: #1a1a2e; border-bottom: 2px solid #eee; padding-bottom: 0.5rem; }
        label { display: block; font-size: 0.85rem; font-weight: 500; margin-bottom: 0.25rem; color: #555; }
        input[type="number"], input[type="range"] {
            width: 100%; padding: 0.5rem; border: 1px solid #ddd; border-radius: 4px;
            font-size: 0.85rem; margin-bottom: 0.75rem;
        }
        input[type="range"] { padding: 0; }
        input[type="number"]:focus { outline: none; border-color: #1a1a2e; }
        .form-row-3 { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 0.75rem; }
        .form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; }
        .slider-group { margin-bottom: 0.75rem; }
        .slider-group .slider-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.25rem; }
        .slider-group .slider-header label { margin-bottom: 0; }
        .slider-group .slider-value { font-weight: 600; font-size: 0.85rem; color: #1a1a2e; }
        .btn { padding: 0.5rem 1rem; border: none; border-radius: 4px; cursor: pointer; font-size: 0.85rem; font-weight: 500; }
        .btn-primary { background: #1a1a2e; color: white; }
        .btn-primary:hover { background: #16213e; }
        .checkbox-row { display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.75rem; }
        .checkbox-row input[type="checkbox"] { width: auto; margin: 0; }
        .text-muted { color: #888; font-size: 0.8rem; }
        .section-label { font-size: 0.9rem; font-weight: 600; color: #444; margin-bottom: 0.5rem; margin-top: 0.5rem; }
        @media (max-width: 768px) { .form-row-3 { grid-template-columns: 1fr; } .form-row { grid-template-columns: 1fr; } }
    </style>
</head>
<body>
    <nav>
        <span class="brand">Lipila Mock Admin</span>
        <a href="/admin/">Dashboard</a>
    </nav>
    <div class="container">
        {{if .Flash}}<div class="flash flash-success">{{.Flash}}</div>{{end}}
        {{if .Error}}<div class="flash flash-error">{{.Error}}</div>{{end}}

        <div class="card">
            <h2>Simulation Config</h2>
            <p class="text-muted" style="margin-bottom:1rem">
                Configure success rates, processing delays, and timeout behaviour for the mock payment simulator.
                Percentages are clamped to 0-100. Delay must be >= 0 seconds.
            </p>
            <form method="POST" action="/admin/config">
                <div class="section-label">Mobile Money Success Rates</div>
                <div class="form-row-3">
                    <div class="slider-group">
                        <div class="slider-header">
                            <label for="mtn_success_rate">MTN</label>
                            <span class="slider-value" id="mtn_val">{{.Config.MtnSuccessRate}}%</span>
                        </div>
                        <input type="range" id="mtn_success_rate" name="mtn_success_rate" value="{{.Config.MtnSuccessRate}}" min="0" max="100" oninput="document.getElementById('mtn_val').textContent=this.value+'%'">
                    </div>
                    <div class="slider-group">
                        <div class="slider-header">
                            <label for="airtel_success_rate">Airtel</label>
                            <span class="slider-value" id="airtel_val">{{.Config.AirtelSuccessRate}}%</span>
                        </div>
                        <input type="range" id="airtel_success_rate" name="airtel_success_rate" value="{{.Config.AirtelSuccessRate}}" min="0" max="100" oninput="document.getElementById('airtel_val').textContent=this.value+'%'">
                    </div>
                    <div class="slider-group">
                        <div class="slider-header">
                            <label for="zamtel_success_rate">Zamtel</label>
                            <span class="slider-value" id="zamtel_val">{{.Config.ZamtelSuccessRate}}%</span>
                        </div>
                        <input type="range" id="zamtel_success_rate" name="zamtel_success_rate" value="{{.Config.ZamtelSuccessRate}}" min="0" max="100" oninput="document.getElementById('zamtel_val').textContent=this.value+'%'">
                    </div>
                </div>

                <div class="section-label">Other Payment Success Rates</div>
                <div class="form-row">
                    <div class="slider-group">
                        <div class="slider-header">
                            <label for="card_success_rate">Card</label>
                            <span class="slider-value" id="card_val">{{.Config.CardSuccessRate}}%</span>
                        </div>
                        <input type="range" id="card_success_rate" name="card_success_rate" value="{{.Config.CardSuccessRate}}" min="0" max="100" oninput="document.getElementById('card_val').textContent=this.value+'%'">
                    </div>
                    <div class="slider-group">
                        <div class="slider-header">
                            <label for="bank_success_rate">Bank</label>
                            <span class="slider-value" id="bank_val">{{.Config.BankSuccessRate}}%</span>
                        </div>
                        <input type="range" id="bank_success_rate" name="bank_success_rate" value="{{.Config.BankSuccessRate}}" min="0" max="100" oninput="document.getElementById('bank_val').textContent=this.value+'%'">
                    </div>
                </div>

                <div class="section-label">Processing</div>
                <div class="form-row">
                    <div>
                        <label for="processing_delay_seconds">Processing Delay (seconds)</label>
                        <input type="number" id="processing_delay_seconds" name="processing_delay_seconds" value="{{.Config.ProcessingDelaySeconds}}" min="0" max="300">
                    </div>
                    <div>
                        <label for="timeout_probability">Timeout Probability (%)</label>
                        <input type="number" id="timeout_probability" name="timeout_probability" value="{{.Config.TimeoutProbability}}" min="0" max="100">
                    </div>
                </div>
                <div class="checkbox-row">
                    <input type="checkbox" id="enable_random_timeouts" name="enable_random_timeouts" {{if .Config.EnableRandomTimeouts}}checked{{end}}>
                    <label for="enable_random_timeouts" style="margin-bottom:0">Enable Random Timeouts</label>
                </div>

                <button type="submit" class="btn btn-primary">Save Config</button>
            </form>
        </div>
    </div>
</body>
</html>`

const resetConfirmHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Reset Database - Lipila Mock Admin</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #333; }
        nav { background: #1a1a2e; color: white; padding: 1rem 2rem; display: flex; gap: 2rem; align-items: center; }
        nav .brand { font-weight: bold; font-size: 1.2rem; }
        nav a { color: #ccc; text-decoration: none; font-size: 0.9rem; }
        nav a:hover { color: white; }
        .container { max-width: 600px; margin: 2rem auto; padding: 0 1rem; }
        .card { background: white; border-radius: 8px; padding: 1.5rem; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
        .card h2 { margin-bottom: 1rem; font-size: 1.1rem; color: #dc3545; border-bottom: 2px solid #f8d7da; padding-bottom: 0.5rem; }
        .warning { background: #fff3cd; border: 1px solid #ffc107; border-radius: 6px; padding: 1rem; margin-bottom: 1rem; font-size: 0.85rem; color: #856404; }
        .warning strong { display: block; margin-bottom: 0.25rem; }
        ul { margin: 0.5rem 0 0 1.25rem; font-size: 0.85rem; color: #666; }
        li { margin-bottom: 0.25rem; }
        .actions { display: flex; gap: 0.75rem; margin-top: 1.25rem; }
        .btn { padding: 0.5rem 1.25rem; border: none; border-radius: 4px; cursor: pointer; font-size: 0.85rem; font-weight: 500; text-decoration: none; display: inline-block; }
        .btn-danger { background: #dc3545; color: white; }
        .btn-danger:hover { background: #c82333; }
        .btn-secondary { background: #6c757d; color: white; }
        .btn-secondary:hover { background: #5a6268; }
    </style>
</head>
<body>
    <nav>
        <span class="brand">Lipila Mock Admin</span>
        <a href="/admin/">Dashboard</a>
    </nav>
    <div class="container">
        <div class="card">
            <h2>Reset Entire Database</h2>
            <div class="warning">
                <strong>Are you sure?</strong>
                This action is irreversible and will permanently delete all data.
            </div>
            <p style="font-size:0.85rem; color:#666; margin-bottom:0.5rem;">The following will be deleted:</p>
            <ul>
                <li>All wallets and their API keys</li>
                <li>All transactions (collections, disbursements)</li>
                <li>All callback logs</li>
                <li>Simulation config (will be reset to defaults)</li>
            </ul>
            <p style="font-size:0.85rem; color:#666; margin-top:0.75rem;">A new default wallet will be created automatically after reset.</p>
            <div class="actions">
                <form method="POST" action="/admin/reset">
                    <button type="submit" class="btn btn-danger">Yes, Reset Everything</button>
                </form>
                <a href="/admin/" class="btn btn-secondary">Cancel</a>
            </div>
        </div>
    </div>
</body>
</html>`
