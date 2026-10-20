async function loadPosts() {
  posts = await api('/api/posts');
  document.querySelector('#post-count').textContent = posts.length;
  renderPosts();
}

function renderPosts() {
  const list = document.querySelector('#post-list');
  list.replaceChildren();
  const search = document.querySelector('#post-search').value.trim().toLowerCase();
  const filtered = posts.filter((item) => {
    const matchesStatus = postFilter === 'all' || item.status === postFilter;
    const matchesSearch = !search || item.title.toLowerCase().includes(search) || item.content.toLowerCase().includes(search) || (item.tags || []).some((tag) => tag.includes(search));
    return matchesStatus && matchesSearch;
  });
  const empty = document.querySelector('#posts-empty');
  empty.hidden = posts.length > 0;
  if (!posts.length) return;
  if (!filtered.length) {
    const emptyFilter = document.createElement('p');
    emptyFilter.className = 'post-filter-empty';
    emptyFilter.textContent = search ? 'No posts match that search.' : `No ${postFilter} posts yet.`;
    list.append(emptyFilter);
    return;
  }
  for (const item of filtered) {
    const button = document.createElement('button');
    button.className = 'post-row';
    button.type = 'button';
    button.addEventListener('click', () => openPost(item.id));
    const title = document.createElement('strong');
    title.textContent = item.title;
    const preview = document.createElement('span');
    preview.className = 'post-row-preview';
    preview.textContent = item.content.slice(0, 110) || 'A blank page, waiting for a first line.';
    const status = document.createElement('span');
    status.className = `post-status ${item.status}`;
    status.textContent = item.status === 'published' ? 'Ready · private' : 'Draft';
    const date = document.createElement('time');
    date.textContent = formatDate(item.updatedAt);
    button.append(title, preview, status, date);
    if (item.tags?.length) {
      const tags = document.createElement('span');
      tags.className = 'content-tag-list post-row-tags';
      tags.textContent = item.tags.map((tag) => `#${tag}`).join(' ');
      button.append(tags);
    }
    list.append(button);
  }
}

async function createPost() {
  try {
    const item = await api('/api/posts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: 'Untitled post', content: '', tags: [], status: 'draft' }),
    });
    posts.unshift(item);
    document.querySelector('#post-count').textContent = posts.length;
    openPost(item.id);
    refreshDashboard().catch(showError);
  } catch (error) {
    showError(error);
  }
}

function openPost(id) {
  const item = posts.find((post) => post.id === id);
  if (!item) return;
  activePostID = id;
  document.querySelector('#post-title').value = item.title;
  document.querySelector('#post-content').value = item.content;
  document.querySelector('#post-tags').value = (item.tags || []).join(', ');
  document.querySelector('#post-status').value = item.status;
  document.querySelector('#post-editor').hidden = false;
  document.querySelector('#post-list').hidden = true;
  document.querySelector('.post-filters').hidden = true;
  document.querySelector('.posts-heading').hidden = true;
  document.querySelector('#posts-empty').hidden = true;
  document.querySelector('#post-save-status').textContent = `Last saved ${formatDate(item.updatedAt)}`;
  closeSidebar();
}

function closePostEditor() {
  activePostID = null;
  document.querySelector('#post-editor').hidden = true;
  document.querySelector('#post-list').hidden = false;
  document.querySelector('.post-filters').hidden = false;
  document.querySelector('.posts-heading').hidden = false;
  renderPosts();
}

async function savePost() {
  const item = posts.find((post) => post.id === activePostID);
  if (!item) return;
  const title = document.querySelector('#post-title').value.trim();
  const content = document.querySelector('#post-content').value;
  const tags = parseTagsInput(document.querySelector('#post-tags').value);
  const status = document.querySelector('#post-status').value;
  if (!title) {
    showToast('Add a title before saving.');
    document.querySelector('#post-title').focus();
    return;
  }
  const button = document.querySelector('#save-post');
  button.disabled = true;
  try {
    await api(`/api/posts/${item.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title, content, tags, status }),
    });
    Object.assign(item, { title, content, tags, status, updatedAt: new Date().toISOString() });
    document.querySelector('#post-save-status').textContent = 'Saved privately just now';
    renderPosts();
    document.querySelector('#post-count').textContent = posts.length;
    refreshDashboard().catch(showError);
    showToast('Post saved privately.');
  } catch (error) {
    showError(error);
  } finally {
    button.disabled = false;
  }
}

async function deletePost() {
  const item = posts.find((post) => post.id === activePostID);
  if (!item || !window.confirm('Delete this post? This cannot be undone.')) return;
  try {
    await api(`/api/posts/${item.id}`, { method: 'DELETE' });
    posts = posts.filter((post) => post.id !== item.id);
    document.querySelector('#post-count').textContent = posts.length;
    closePostEditor();
    refreshDashboard().catch(showError);
    showToast('Post deleted.');
  } catch (error) {
    showError(error);
  }
}
