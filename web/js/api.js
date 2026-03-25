const API = {
  sessionToken: null,

  async connect(clientId, clientSecret, saveToKeyring) {
    const data = await this._post('/api/connect', { clientId, clientSecret, saveToKeyring: !!saveToKeyring });
    this.sessionToken = data.sessionToken || null;
    return data;
  },
  async connectFromKeyring() {
    const data = await this._post('/api/connect', { useKeyring: true });
    this.sessionToken = data.sessionToken || null;
    return data;
  },
  async getCredentialStatus() {
    return this._get('/api/credentials');
  },
  async clearCredentials() {
    return this._fetch('/api/credentials', { method: 'DELETE' });
  },
  async getVersion() {
    return this._get('/api/version');
  },
  async getDataSources() {
    return this._get('/api/datasources');
  },
  async getTemplates() {
    return this._get('/api/templates');
  },
  async saveTemplate(template) {
    return this._post('/api/templates', template);
  },
  async updateTemplate(id, template) {
    return this._fetch('/api/templates/' + id, { method: 'PUT', body: JSON.stringify(template) });
  },
  async deleteTemplate(id) {
    return this._fetch('/api/templates/' + id, { method: 'DELETE' });
  },
  async cloneTemplate(id, name) {
    return this._post('/api/templates/' + id + '/clone', { name });
  },
  async importTemplate(template) {
    return this._post('/api/templates/import', template);
  },
  async preview(config) {
    return this._post('/api/preview', config);
  },
  async generate(config) {
    return this._post('/api/generate', config);
  },
  async viewLoad(config) {
    return this._post('/api/view', config);
  },
  async viewPage(page, pageSize, search) {
    let url = `/api/view?page=${page}&pageSize=${pageSize || 100}`;
    if (search) url += '&search=' + encodeURIComponent(search);
    return this._get(url);
  },
  async getHistory() {
    return this._get('/api/history');
  },
  async cleanup() {
    return this._post('/api/cleanup', {});
  },
  downloadUrl(filename) {
    // Include session token as query param for download links
    let url = '/api/download/' + encodeURIComponent(filename);
    if (this.sessionToken) url += '?token=' + encodeURIComponent(this.sessionToken);
    return url;
  },
  exportTemplateUrl(id) {
    return '/api/templates/' + encodeURIComponent(id);
  },
  _headers() {
    const h = { 'Content-Type': 'application/json' };
    if (this.sessionToken) h['X-Session-Token'] = this.sessionToken;
    return h;
  },
  async _get(url) {
    const opts = { headers: this._headers() };
    const resp = await fetch(url, opts);
    if (resp.status === 401) { this.sessionToken = null; App.onSessionExpired(); throw new Error('Session expired'); }
    const data = await resp.json();
    if (!resp.ok) throw new Error(data.error || 'Request failed');
    return data;
  },
  async _post(url, body) {
    return this._fetch(url, { method: 'POST', body: JSON.stringify(body) });
  },
  async _fetch(url, opts) {
    opts.headers = this._headers();
    const resp = await fetch(url, opts);
    if (resp.status === 401) { this.sessionToken = null; App.onSessionExpired(); throw new Error('Session expired'); }
    const data = await resp.json();
    if (!resp.ok) throw new Error(data.error || 'Request failed');
    return data;
  }
};
