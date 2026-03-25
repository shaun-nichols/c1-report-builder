const Templates = {
  all: [],
  searchQuery: '',

  async load() {
    this.all = await API.getTemplates();
    this.render();
  },

  setSearch(q) {
    this.searchQuery = q.toLowerCase();
    this.render();
  },

  render() {
    const q = this.searchQuery;
    const filtered = q
      ? this.all.filter(t =>
          t.name.toLowerCase().includes(q) ||
          (t.description || '').toLowerCase().includes(q) ||
          t.dataSource.toLowerCase().includes(q) ||
          (t.category || '').toLowerCase().includes(q))
      : this.all;

    const builtin = filtered.filter(t => t.builtin);
    const user = filtered.filter(t => !t.builtin);

    // Group builtins by category
    const categories = {};
    builtin.forEach(t => {
      const cat = t.category || 'Other';
      if (!categories[cat]) categories[cat] = [];
      categories[cat].push(t);
    });

    const builtinEl = document.getElementById('builtin-templates');
    builtinEl.innerHTML = '';
    const catOrder = ['Data Audit', 'Governance', 'Operations', 'Other'];
    catOrder.forEach(cat => {
      if (!categories[cat] || categories[cat].length === 0) return;
      const header = document.createElement('div');
      header.className = 'category-header';
      header.textContent = cat;
      builtinEl.appendChild(header);

      const grid = document.createElement('div');
      grid.className = 'template-grid';
      this.renderCards(grid, categories[cat], true);
      builtinEl.appendChild(grid);
    });

    const userEl = document.getElementById('user-templates');
    userEl.innerHTML = '';
    if (user.length > 0) {
      const grid = document.createElement('div');
      grid.className = 'template-grid';
      this.renderCards(grid, user, false);
      userEl.appendChild(grid);
    }

    const empty = document.getElementById('no-user-templates');
    if (user.length === 0 && !q) {
      empty.classList.remove('hidden');
    } else {
      empty.classList.add('hidden');
    }
  },

  renderCards(container, templates, isBuiltin) {
    templates.forEach(t => {
      const card = document.createElement('div');
      card.className = 'template-card';

      let actions = `<button class="btn btn-primary btn-sm" onclick="event.stopPropagation(); App.runTemplate('${t.id}')">Run</button>`;
      actions += `<button class="btn btn-ghost btn-sm" onclick="event.stopPropagation(); App.cloneTemplate('${t.id}')">Clone</button>`;
      actions += `<button class="btn btn-ghost btn-sm" onclick="event.stopPropagation(); App.exportTemplate('${t.id}')">Export</button>`;

      if (!isBuiltin) {
        actions += `<button class="btn btn-ghost btn-sm" onclick="event.stopPropagation(); App.editTemplate('${t.id}')">Edit</button>`;
        actions += `<button class="btn btn-danger btn-sm" onclick="event.stopPropagation(); App.deleteTemplate('${t.id}')">Delete</button>`;
      }

      const lastRun = App.lastRunInfo[t.id];
      const lastRunHtml = lastRun
        ? `<div class="tc-lastrun">Last run: ${esc(lastRun.rows)} rows at ${esc(lastRun.time)}</div>`
        : '';

      card.innerHTML = `
        <div class="tc-name">${esc(t.name)}</div>
        <div class="tc-desc">${esc(t.description || '')}</div>
        <div class="tc-meta">
          <span>${esc(t.dataSource)}</span>
          <span>${(t.columns || []).length} columns</span>
          ${(t.filters || []).length > 0 ? `<span>${t.filters.length} filters</span>` : ''}
        </div>
        ${lastRunHtml}
        <div class="tc-actions">${actions}</div>
      `;
      card.onclick = () => App.openTemplate(t.id);
      container.appendChild(card);
    });
  }
};
