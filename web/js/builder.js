// Friendly operator labels
const OP_LABELS = {
  eq: 'equals', neq: 'does not equal', contains: 'contains', not_contains: 'does not contain',
  empty: 'is empty', not_empty: 'is not empty', gt: 'greater than', lt: 'less than',
  before: 'before', after: 'after'
};

const OPS_BY_TYPE = {
  string:  ['eq', 'neq', 'contains', 'not_contains', 'empty', 'not_empty'],
  enum:    ['eq', 'neq', 'empty', 'not_empty'],
  date:    ['eq', 'before', 'after', 'empty', 'not_empty'],
  number:  ['eq', 'neq', 'gt', 'lt', 'empty', 'not_empty'],
  boolean: ['eq']
};

const DATE_PRESETS = [
  { label: 'Last 7 days', value: () => daysAgo(7) },
  { label: 'Last 30 days', value: () => daysAgo(30) },
  { label: 'Last 90 days', value: () => daysAgo(90) },
  { label: 'This year', value: () => new Date(new Date().getFullYear(), 0, 1).toISOString() },
];

function daysAgo(n) {
  const d = new Date(); d.setDate(d.getDate() - n);
  return d.toISOString();
}

const Builder = {
  dataSources: [],
  currentDS: null,
  columns: [],
  filters: [],
  apps: [],
  columnSearch: '',

  init(dataSources, apps) {
    this.dataSources = dataSources;
    this.apps = apps;

    const sel = document.getElementById('ds-select');
    sel.innerHTML = '<option value="">-- Select a data source --</option>';
    dataSources.forEach(ds => {
      const opt = document.createElement('option');
      opt.value = ds.id;
      opt.textContent = ds.label;
      sel.appendChild(opt);
    });

    const appSel = document.getElementById('app-select');
    appSel.innerHTML = '<option value="all">All Applications</option>';
    apps.forEach(app => {
      const opt = document.createElement('option');
      opt.value = app.id;
      opt.textContent = app.displayName;
      appSel.appendChild(opt);
    });
  },

  setDataSource(dsId) {
    document.getElementById('ds-select').value = dsId;
    this.onDataSourceChange();
  },

  onDataSourceChange() {
    const dsId = document.getElementById('ds-select').value;
    this.currentDS = this.dataSources.find(ds => ds.id === dsId) || null;

    const appGroup = document.getElementById('app-select-group');
    const emptyState = document.getElementById('builder-empty');
    const builderContent = document.getElementById('builder-content');

    if (this.currentDS) {
      if (this.currentDS.requiresApp) appGroup.classList.remove('hidden');
      else appGroup.classList.add('hidden');
      this.columns = this.currentDS.columns.map(c => ({ id: c.id, label: c.label, checked: true }));
      this.filters = [];
      this.columnSearch = '';
      if (emptyState) emptyState.classList.add('hidden');
      if (builderContent) builderContent.classList.remove('hidden');
    } else {
      appGroup.classList.add('hidden');
      this.columns = [];
      this.filters = [];
      if (emptyState) emptyState.classList.remove('hidden');
      if (builderContent) builderContent.classList.add('hidden');
    }

    this.renderColumns();
    this.renderFilters();
    this.renderSortOptions();
    this.hidePreview();
  },

  loadFromTemplate(t) {
    this.setDataSource(t.dataSource);

    if (t.columns && t.columns.length > 0) {
      const templateCols = new Set(t.columns);
      const ordered = [];
      t.columns.forEach(cid => {
        const col = this.columns.find(c => c.id === cid);
        if (col) ordered.push({ ...col, checked: true });
      });
      this.columns.forEach(c => {
        if (!templateCols.has(c.id)) ordered.push({ ...c, checked: false });
      });
      this.columns = ordered;
    }

    this.filters = (t.filters || []).map(f => ({ ...f }));
    this.columnSearch = '';

    this.renderColumns();
    this.renderFilters();
    this.renderSortOptions();

    if (t.sortBy) {
      document.getElementById('sort-by').value = t.sortBy;
      document.getElementById('sort-dir').value = t.sortDesc ? 'desc' : 'asc';
    }
    if (t.format) document.getElementById('format-select').value = t.format;
    document.getElementById('report-name').value = t.name || '';
    document.getElementById('report-desc').value = t.description || '';
  },

  // --- Columns ---
  renderColumns() {
    const el = document.getElementById('column-picker');
    el.innerHTML = '';
    const search = this.columnSearch.toLowerCase();

    this.columns.forEach((col, idx) => {
      if (search && !col.label.toLowerCase().includes(search) && !col.id.toLowerCase().includes(search)) return;
      const div = document.createElement('div');
      div.className = 'col-item';
      div.innerHTML = `
        <input type="checkbox" ${col.checked ? 'checked' : ''} onchange="Builder.toggleColumn(${idx})">
        <span class="col-label">${esc(col.label)}</span>
        <span class="col-arrows">
          <button onclick="event.stopPropagation(); Builder.moveColumn(${idx}, -1)">&uarr;</button>
          <button onclick="event.stopPropagation(); Builder.moveColumn(${idx}, 1)">&darr;</button>
        </span>
      `;
      el.appendChild(div);
    });
  },

  onColumnSearch(val) {
    this.columnSearch = val;
    this.renderColumns();
  },

  selectAllColumns(checked) {
    this.columns.forEach(c => c.checked = checked);
    this.renderColumns();
  },

  toggleColumn(idx) {
    this.columns[idx].checked = !this.columns[idx].checked;
  },

  moveColumn(idx, dir) {
    const newIdx = idx + dir;
    if (newIdx < 0 || newIdx >= this.columns.length) return;
    [this.columns[idx], this.columns[newIdx]] = [this.columns[newIdx], this.columns[idx]];
    this.renderColumns();
  },

  // --- Filters ---
  getColumnDef(colId) {
    if (!this.currentDS) return null;
    return this.currentDS.columns.find(c => c.id === colId) || null;
  },

  renderFilters() {
    const el = document.getElementById('filter-list');
    el.innerHTML = '';
    this.filters.forEach((f, idx) => {
      const colDef = this.getColumnDef(f.columnId);
      const colType = colDef ? colDef.type : 'string';
      const ops = OPS_BY_TYPE[colType] || OPS_BY_TYPE.string;

      const div = document.createElement('div');
      div.className = 'filter-row';

      // Column select
      const colOpts = this.currentDS ? this.currentDS.columns.map(c =>
        `<option value="${c.id}" ${f.columnId === c.id ? 'selected' : ''}>${esc(c.label)}</option>`
      ).join('') : '';

      // Operator select
      const opOpts = ops.map(op =>
        `<option value="${op}" ${f.operator === op ? 'selected' : ''}>${OP_LABELS[op] || op}</option>`
      ).join('');

      // Value input — varies by type
      let valueHtml = '';
      const needsValue = f.operator !== 'empty' && f.operator !== 'not_empty';

      if (!needsValue) {
        valueHtml = '<span style="color:var(--text-muted); font-size:0.8em; padding:0 4px;">—</span>';
      } else if (colDef && colDef.options && colDef.options.length > 0) {
        const valOpts = colDef.options.map(o =>
          `<option value="${o}" ${f.value === o ? 'selected' : ''}>${esc(o)}</option>`
        ).join('');
        valueHtml = `<select onchange="Builder.updateFilter(${idx}, 'value', this.value)"><option value="">—</option>${valOpts}</select>`;
      } else if (colType === 'boolean') {
        valueHtml = `<select onchange="Builder.updateFilter(${idx}, 'value', this.value)">
          <option value="Yes" ${f.value === 'Yes' ? 'selected' : ''}>Yes</option>
          <option value="No" ${f.value === 'No' ? 'selected' : ''}>No</option>
        </select>`;
      } else if (colType === 'date' && (f.operator === 'after' || f.operator === 'before')) {
        const presetOpts = DATE_PRESETS.map(p =>
          `<option value="${p.label}">${p.label}</option>`
        ).join('');
        valueHtml = `<input value="${esc(f.value || '')}" oninput="Builder.updateFilter(${idx}, 'value', this.value)" placeholder="ISO date or preset">`;
        // Add preset buttons below
        valueHtml += `<div class="filter-presets">${DATE_PRESETS.map(p =>
          `<button class="btn-preset" onclick="Builder.setFilterPreset(${idx}, '${p.label}')">${p.label}</button>`
        ).join('')}</div>`;
      } else {
        valueHtml = `<input value="${esc(f.value || '')}" oninput="Builder.updateFilter(${idx}, 'value', this.value)">`;
      }

      div.innerHTML = `
        <select onchange="Builder.onFilterColumnChange(${idx}, this.value)">${colOpts}</select>
        <select onchange="Builder.updateFilter(${idx}, 'operator', this.value)">${opOpts}</select>
        <div class="filter-value-wrap">${valueHtml}</div>
        <button class="btn-remove" onclick="Builder.removeFilter(${idx})">&times;</button>
      `;
      el.appendChild(div);
    });
  },

  addFilter() {
    if (!this.currentDS || this.currentDS.columns.length === 0) return;
    this.filters.push({ columnId: this.currentDS.columns[0].id, operator: 'eq', value: '' });
    this.renderFilters();
  },

  updateFilter(idx, field, value) {
    this.filters[idx][field] = value;
    // Re-render if operator changed (to toggle value visibility)
    if (field === 'operator') this.renderFilters();
  },

  onFilterColumnChange(idx, colId) {
    this.filters[idx].columnId = colId;
    this.filters[idx].operator = 'eq';
    this.filters[idx].value = '';
    this.renderFilters();
  },

  setFilterPreset(idx, label) {
    const preset = DATE_PRESETS.find(p => p.label === label);
    if (preset) {
      this.filters[idx].value = preset.value();
      this.renderFilters();
    }
  },

  removeFilter(idx) {
    this.filters.splice(idx, 1);
    this.renderFilters();
  },

  // --- Sort ---
  renderSortOptions() {
    const sel = document.getElementById('sort-by');
    const current = sel.value;
    sel.innerHTML = '<option value="">None</option>';
    if (this.currentDS) {
      this.currentDS.columns.forEach(c => {
        sel.appendChild(Object.assign(document.createElement('option'), { value: c.id, textContent: c.label }));
      });
    }
    sel.value = current;
  },

  // --- Config ---
  getConfig() {
    const appSel = document.getElementById('app-select');
    const appVal = appSel ? appSel.value : '';
    let appId = '';
    let appIds = [];
    if (this.currentDS?.requiresApp) {
      if (appVal === 'all') {
        appIds = ['all'];
      } else if (appVal) {
        appId = appVal;
      }
    }

    return {
      name: document.getElementById('report-name').value || 'Custom Report',
      description: document.getElementById('report-desc').value || '',
      dataSource: document.getElementById('ds-select').value,
      appId,
      appIds,
      columns: this.columns.filter(c => c.checked).map(c => c.id),
      filters: this.filters.filter(f => f.columnId && (f.operator === 'empty' || f.operator === 'not_empty' || f.value)),
      sortBy: document.getElementById('sort-by').value,
      sortDesc: document.getElementById('sort-dir').value === 'desc',
      format: document.getElementById('format-select').value,
    };
  },

  // --- Preview / Download ---
  showPreview(headers, rows, total, truncated) {
    document.getElementById('preview-area').classList.remove('hidden');
    document.getElementById('download-area').classList.add('hidden');
    document.getElementById('viewer-area').classList.add('hidden');

    if (rows.length === 0) {
      document.getElementById('preview-info').textContent = '';
      document.getElementById('preview-thead').innerHTML = '';
      document.getElementById('preview-tbody').innerHTML = `
        <tr><td colspan="99" class="empty-table-msg">
          No results found. Try adjusting your filters or selecting a different application.
        </td></tr>`;
      return;
    }

    let info = `Showing ${rows.length} rows`;
    if (truncated) info += ` of ${total}+ total`;
    document.getElementById('preview-info').textContent = info;

    const thead = document.getElementById('preview-thead');
    thead.innerHTML = '<tr>' + headers.map(h => `<th>${esc(h)}</th>`).join('') + '</tr>';

    const tbody = document.getElementById('preview-tbody');
    tbody.innerHTML = '';
    rows.forEach(row => {
      const tr = document.createElement('tr');
      tr.innerHTML = headers.map(h => {
        const raw = row[h] || '';
        const display = formatCell(raw);
        return `<td title="${esc(raw)}">${esc(display)}</td>`;
      }).join('');
      tbody.appendChild(tr);
    });
  },

  hidePreview() {
    document.getElementById('preview-area').classList.add('hidden');
    document.getElementById('download-area').classList.add('hidden');
  },

  // --- Full Report Viewer ---
  viewerPage: 1,
  viewerSearch: '',

  showViewer() {
    document.getElementById('preview-area').classList.add('hidden');
    document.getElementById('download-area').classList.add('hidden');
    document.getElementById('viewer-area').classList.remove('hidden');
  },

  hideViewer() {
    document.getElementById('viewer-area').classList.add('hidden');
  },

  async loadViewer(config) {
    this.viewerPage = 1;
    this.viewerSearch = '';
    document.getElementById('viewer-search').value = '';
    await API.viewLoad(config);
    await this.renderViewerPage();
    this.showViewer();
  },

  async renderViewerPage() {
    const data = await API.viewPage(this.viewerPage, 100, this.viewerSearch);

    // Headers
    const thead = document.getElementById('viewer-thead');
    thead.innerHTML = '<tr>' + data.headers.map(h => `<th>${esc(h)}</th>`).join('') + '</tr>';

    // Body
    const tbody = document.getElementById('viewer-tbody');
    tbody.innerHTML = '';
    if (!data.rows || data.rows.length === 0) {
      tbody.innerHTML = `<tr><td colspan="${data.headers.length}" class="empty-table-msg">
        ${this.viewerSearch ? 'No rows match your search.' : 'No results found.'}
      </td></tr>`;
    } else {
      data.rows.forEach(row => {
        const tr = document.createElement('tr');
        tr.innerHTML = data.headers.map(h => {
          const raw = row[h] || '';
          const display = formatCell(raw);
          return `<td title="${esc(raw)}">${esc(display)}</td>`;
        }).join('');
        tbody.appendChild(tr);
      });
    }

    // Pagination info
    const info = document.getElementById('viewer-info');
    const start = (data.page - 1) * data.pageSize + 1;
    const end = Math.min(data.page * data.pageSize, data.total);
    info.textContent = data.total > 0
      ? `Showing ${start}-${end} of ${data.total} rows`
      : 'No matching rows';

    // Pagination controls
    const controls = document.getElementById('viewer-pagination');
    controls.innerHTML = '';

    const prevBtn = document.createElement('button');
    prevBtn.className = 'btn btn-ghost btn-sm';
    prevBtn.textContent = 'Previous';
    prevBtn.disabled = data.page <= 1;
    prevBtn.onclick = () => { this.viewerPage--; this.renderViewerPage(); };
    controls.appendChild(prevBtn);

    const pageInfo = document.createElement('span');
    pageInfo.className = 'pagination-info';
    pageInfo.textContent = `Page ${data.page} of ${data.totalPages}`;
    controls.appendChild(pageInfo);

    const nextBtn = document.createElement('button');
    nextBtn.className = 'btn btn-ghost btn-sm';
    nextBtn.textContent = 'Next';
    nextBtn.disabled = data.page >= data.totalPages;
    nextBtn.onclick = () => { this.viewerPage++; this.renderViewerPage(); };
    controls.appendChild(nextBtn);
  },

  onViewerSearch(val) {
    this.viewerSearch = val;
    this.viewerPage = 1;
    // Debounce
    clearTimeout(this._viewerSearchTimer);
    this._viewerSearchTimer = setTimeout(() => this.renderViewerPage(), 300);
  },

  _lastGenerateConfig: null,

  showDownload(files, hash, metadata, config) {
    document.getElementById('preview-area').classList.add('hidden');
    document.getElementById('viewer-area').classList.add('hidden');
    document.getElementById('download-area').classList.remove('hidden');
    this._lastGenerateConfig = config || null;

    let html = '<div style="margin-bottom:12px; font-size:0.9em; color:var(--text-light);">Report generated successfully</div>';
    files.forEach(f => {
      html += `<a class="download-link" href="${API.downloadUrl(f)}" download="${esc(f)}">Download ${esc(f)}</a> `;
    });
    html += '<button class="btn btn-secondary" style="margin-left:8px; vertical-align:middle;" onclick="Builder.openViewer()">View Full Report</button>';
    if (metadata && metadata['Total Rows']) {
      html += `<div class="hash-display">${metadata['Total Rows']} rows</div>`;
    }
    // Show summary stats
    if (metadata) {
      const summaries = Object.entries(metadata).filter(([k]) => k.startsWith('Summary:'));
      if (summaries.length > 0) {
        html += '<div class="summary-stats">';
        summaries.forEach(([k, v]) => {
          html += `<div class="summary-item"><span class="summary-label">${esc(k.replace('Summary: ', ''))}</span> ${esc(v)}</div>`;
        });
        html += '</div>';
      }
    }
    if (hash) {
      html += `<div class="hash-display">SHA-256: <code>${hash}</code></div>`;
    }
    document.getElementById('download-box').innerHTML = html;
  },

  async openViewer() {
    const config = this._lastGenerateConfig || this.getConfig();
    setLoading('builder-status', true, 'Loading full report...');
    try {
      await this.loadViewer(config);
      clearStatus('builder-status');
    } catch (e) {
      showStatus('builder-status', 'Failed to load viewer: ' + e.message, 'error');
    }
  }
};

function esc(s) {
  if (!s) return '';
  const d = document.createElement('div');
  d.textContent = String(s);
  return d.innerHTML;
}

// Format ISO timestamps to readable dates for display
function formatCell(val) {
  if (!val) return '';
  // Detect ISO 8601 timestamps
  if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}/.test(val)) {
    try {
      const d = new Date(val);
      if (!isNaN(d.getTime())) {
        return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
          + ' ' + d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
      }
    } catch { /* fall through */ }
  }
  return val;
}
