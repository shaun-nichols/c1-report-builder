const App = {
  apps: [],
  dataSources: [],
  lastRunInfo: {}, // templateId -> { rows, time }

  showPage(id) {
    document.querySelectorAll('.page').forEach(p => p.classList.add('hidden'));
    document.getElementById('page-' + id).classList.remove('hidden');
  },

  async showDashboard() {
    this.showPage('dashboard');
    await Templates.load();
    this.loadHistory();
  },

  // --- Session expired ---
  onSessionExpired() {
    this.showPage('connect');
    showStatus('connect-status', 'Session expired. Please reconnect.', 'error');
  },

  // --- Connect ---
  async checkSavedCredentials() {
    try {
      const status = await API.getCredentialStatus();
      if (status.keyringAvailable) {
        document.getElementById('keyring-option').classList.remove('hidden');
        document.getElementById('keyring-name-option').textContent = status.keyringName;
        if (status.hasSavedCredentials) {
          document.getElementById('keyring-connect').classList.remove('hidden');
          document.getElementById('manual-connect').classList.add('hidden');
          document.getElementById('keyring-name').textContent = status.keyringName;
        }
      }
    } catch { /* keyring not available, show manual only */ }
  },

  async connect() {
    const clientId = document.getElementById('clientId').value.trim();
    const clientSecret = document.getElementById('clientSecret').value.trim();
    if (!clientId || !clientSecret) {
      showStatus('connect-status', 'Both fields are required.', 'error');
      return;
    }

    const saveToKeyring = document.getElementById('save-to-keyring')?.checked || false;

    showStatus('connect-status', 'Connecting...', 'info');
    try {
      const data = await API.connect(clientId, clientSecret, saveToKeyring);
      this.onConnected(data);
    } catch (e) {
      showStatus('connect-status', e.message, 'error');
    }
  },

  async connectFromKeyring() {
    showStatus('connect-status', 'Connecting with saved credentials...', 'info');
    try {
      const data = await API.connectFromKeyring();
      this.onConnected(data);
    } catch (e) {
      showStatus('connect-status', e.message, 'error');
      // Show manual entry as fallback
      document.getElementById('keyring-connect').classList.add('hidden');
      document.getElementById('manual-connect').classList.remove('hidden');
    }
  },

  async clearSavedCredentials() {
    try {
      await API.clearCredentials();
      document.getElementById('keyring-connect').classList.add('hidden');
      document.getElementById('manual-connect').classList.remove('hidden');
      showStatus('connect-status', 'Saved credentials removed.', 'success');
    } catch (e) {
      showStatus('connect-status', 'Failed to clear: ' + e.message, 'error');
    }
  },

  async onConnected(data) {
    this.apps = data.apps || [];
    try {
      const ver = await API.getVersion();
      document.getElementById('tenant-info').textContent = `Connected (${this.apps.length} apps) \u00b7 v${ver.version}`;
    } catch {
      document.getElementById('tenant-info').textContent = `Connected (${this.apps.length} apps)`;
    }
    this.dataSources = await API.getDataSources();
    Builder.init(this.dataSources, this.apps);
    this.showDashboard();
  },

  // --- Report History ---
  async loadHistory() {
    try {
      const history = await API.getHistory();
      const el = document.getElementById('history-list');
      if (!history || history.length === 0) {
        el.innerHTML = '<div class="empty-state" style="padding:1em">No reports generated this session.</div>';
        return;
      }
      el.innerHTML = '';
      history.forEach(h => {
        const div = document.createElement('div');
        div.className = 'history-item';
        let filesHtml = (h.files || []).map(f =>
          `<a href="${API.downloadUrl(f)}" download="${esc(f)}" class="history-dl">${esc(f)}</a>`
        ).join(' ');
        div.innerHTML = `
          <div class="history-name">${esc(h.name)} <span class="history-meta">${esc(h.format.toUpperCase())} \u00b7 ${esc(h.rows)} rows \u00b7 ${esc(h.timestamp)}</span></div>
          <div>${filesHtml}</div>
        `;
        el.appendChild(div);
      });
    } catch { /* ignore */ }
  },

  async cleanupOutput() {
    try {
      const result = await API.cleanup();
      const n = result.deleted || 0;
      alert(n > 0 ? `Deleted ${n} old file(s).` : 'No old files to clean up.');
    } catch (e) { alert('Cleanup failed: ' + e.message); }
  },

  // --- Report Builder ---
  newReport() {
    document.getElementById('builder-title').textContent = 'New Report';
    document.getElementById('report-name').value = '';
    document.getElementById('report-desc').value = '';
    document.getElementById('ds-select').value = '';
    Builder.onDataSourceChange();
    Builder.hidePreview();
    clearStatus('builder-status');
    this.showPage('builder');
  },

  openTemplate(id) {
    const t = Templates.all.find(t => t.id === id);
    if (!t) return;
    document.getElementById('builder-title').textContent = t.name;
    Builder.loadFromTemplate(t);
    Builder.hidePreview();
    clearStatus('builder-status');
    this.showPage('builder');
  },

  editTemplate(id) { this.openTemplate(id); },

  async runTemplate(id) {
    const t = Templates.all.find(t => t.id === id);
    if (!t) return;
    const ds = this.dataSources.find(d => d.id === t.dataSource);
    if (ds && ds.requiresApp) {
      this.openTemplate(id);
      showStatus('builder-status', 'Select an application and click Generate Report.', 'info');
      return;
    }
    this.openTemplate(id);
    this.generate();
  },

  async cloneTemplate(id) {
    const name = prompt('Name for your copy:');
    if (!name) return;
    try {
      await API.cloneTemplate(id, name);
      await Templates.load();
    } catch (e) { alert('Error: ' + e.message); }
  },

  async deleteTemplate(id) {
    if (!confirm('Delete this template?')) return;
    try {
      await API.deleteTemplate(id);
      await Templates.load();
    } catch (e) { alert('Error: ' + e.message); }
  },

  async exportTemplate(id) {
    try {
      const resp = await fetch(API.exportTemplateUrl(id));
      const data = await resp.json();
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
      const a = document.createElement('a');
      a.href = URL.createObjectURL(blob);
      a.download = (data.name || id).replace(/[^a-zA-Z0-9_-]/g, '_') + '.json';
      a.click();
      URL.revokeObjectURL(a.href);
    } catch (e) { alert('Export failed: ' + e.message); }
  },

  async importTemplate() {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = '.json';
    input.onchange = async (e) => {
      const file = e.target.files[0];
      if (!file) return;
      try {
        const text = await file.text();
        const template = JSON.parse(text);
        await API.importTemplate(template);
        await Templates.load();
      } catch (err) { alert('Import failed: ' + err.message); }
    };
    input.click();
  },

  // --- Preview & Generate ---
  async preview() {
    const config = Builder.getConfig();
    if (!config.dataSource) {
      showStatus('builder-status', 'Select a data source first.', 'error');
      return;
    }
    setLoading('builder-status', true, 'Loading preview...');
    try {
      const data = await API.preview(config);
      Builder.showPreview(data.headers, data.rows, data.total, data.truncated);
      clearStatus('builder-status');
    } catch (e) {
      showStatus('builder-status', 'Preview failed: ' + e.message, 'error');
    }
  },

  async generate() {
    const config = Builder.getConfig();
    if (!config.dataSource) {
      showStatus('builder-status', 'Select a data source first.', 'error');
      return;
    }

    // Confirmation for large reports
    const isAllApps = config.appIds && config.appIds[0] === 'all';
    const ds = this.dataSources.find(d => d.id === config.dataSource);
    const heavySources = ['grants', 'system_log', 'grant_feed', 'past_grants'];

    if (isAllApps && heavySources.includes(config.dataSource)) {
      if (!confirm(`This will fetch ${config.dataSource} data from all ${this.apps.length} apps. This could take several minutes and make thousands of API calls.\n\nContinue?`)) {
        return;
      }
    } else if (isAllApps) {
      if (!confirm(`This will generate a report across all ${this.apps.length} apps. Continue?`)) {
        return;
      }
    }

    setLoading('builder-status', true, 'Generating report... This may take a moment for large datasets.');
    try {
      const data = await API.generate(config);
      Builder.showDownload(data.files, data.hash, data.metadata, config);
      // Track last run for template cards
      const activeTemplate = Templates.all.find(t => t.name === config.name);
      if (activeTemplate) {
        this.lastRunInfo[activeTemplate.id] = {
          rows: data.metadata['Total Rows'] || '?',
          time: new Date().toLocaleTimeString(),
        };
      }
      clearStatus('builder-status');
    } catch (e) {
      showStatus('builder-status', 'Error: ' + e.message, 'error');
    }
  },

  async saveTemplate() {
    const config = Builder.getConfig();
    if (!config.name || config.name === 'Custom Report') {
      showStatus('builder-status', 'Enter a report name before saving.', 'error');
      return;
    }
    if (!config.dataSource) {
      showStatus('builder-status', 'Select a data source before saving.', 'error');
      return;
    }
    try {
      await API.saveTemplate({
        name: config.name,
        description: config.description,
        dataSource: config.dataSource,
        columns: config.columns,
        filters: config.filters,
        sortBy: config.sortBy,
        sortDesc: config.sortDesc,
        format: config.format,
      });
      showStatus('builder-status', 'Template saved!', 'success');
      await Templates.load();
    } catch (e) {
      showStatus('builder-status', 'Save failed: ' + e.message, 'error');
    }
  }
};

// --- Status helpers ---
function showStatus(id, msg, type) {
  const el = document.getElementById(id);
  el.textContent = msg;
  el.className = 'status ' + type;
}

function clearStatus(id) {
  const el = document.getElementById(id);
  el.className = 'status';
  el.textContent = '';
}

function setLoading(id, loading, msg) {
  const el = document.getElementById(id);
  if (loading) {
    el.innerHTML = '<span class="spinner"></span> ' + (msg || 'Loading...');
    el.className = 'status info';
  } else {
    el.className = 'status';
    el.textContent = '';
  }
}

// --- Init ---
document.addEventListener('DOMContentLoaded', () => {
  App.checkSavedCredentials();
});

// --- Keyboard shortcuts ---
document.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') {
    const active = document.activeElement;
    if (active && (active.id === 'clientId' || active.id === 'clientSecret')) {
      e.preventDefault();
      App.connect();
    }
  }
});
