async function loadFiles() {
  files = await api('/api/files');
  document.querySelector('#file-count').textContent = files.length;
  renderFiles();
}
