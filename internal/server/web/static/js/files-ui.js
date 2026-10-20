function renderFiles() {
  const list = document.querySelector('#file-list');
  list.replaceChildren();
  document.querySelector('#empty-files').hidden = files.length !== 0;
  for (const file of files) {
    const row = document.createElement('div');
    row.className = 'file-row';
    const nameCell = document.createElement('div');
    nameCell.className = 'file-name-cell';
    const type = document.createElement('span');
    type.className = 'file-type-icon';
    type.textContent = (file.name.split('.').pop() || 'FILE').slice(0, 4).toUpperCase();
    const name = document.createElement('span');
    name.className = 'file-name-text';
    name.textContent = file.name;
    nameCell.append(type, name);
    const size = document.createElement('span');
    size.textContent = formatSize(file.size);
    const date = document.createElement('span');
    date.textContent = formatDate(file.createdAt);
    const download = document.createElement('button');
    download.className = 'download-button';
    download.type = 'button';
    download.title = `Download ${file.name}`;
    download.setAttribute('aria-label', `Download ${file.name}`);
    download.innerHTML = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3v12m0 0 4-4m-4 4-4-4M5 17v3h14v-3"/></svg>';
    download.addEventListener('click', () => { window.location.href = `/api/files/${file.id}/download`; });
    row.append(nameCell, size, date, download);
    list.append(row);
  }
}

function formatSize(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

async function uploadFiles(fileList) {
  const selected = [...fileList];
  if (!selected.length) return;
  for (const file of selected) {
    if (file.size > 25 * 1024 * 1024) {
      showToast(`${file.name} is larger than 25 MB.`);
      continue;
    }
    const formData = new FormData();
    formData.append('file', file);
    try {
      const uploaded = await api('/api/files', { method: 'POST', body: formData });
      files.unshift(uploaded);
    } catch (error) {
      showToast(error.message);
    }
  }
  document.querySelector('#file-count').textContent = files.length;
  renderFiles();
  document.querySelector('#file-input').value = '';
}
