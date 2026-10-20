const authScreen = document.querySelector('#auth-screen');
const workspace = document.querySelector('#workspace');
const authForm = document.querySelector('#auth-form');
const authError = document.querySelector('#auth-error');
const authSubmit = document.querySelector('#auth-submit');
const authTabs = [...document.querySelectorAll('.auth-tab')];
const editorCanvas = document.querySelector('#editor-canvas');
const noteTitle = document.querySelector('#note-title');
const saveState = document.querySelector('#save-state');
const toast = document.querySelector('#toast');

let authMode = 'signin';
let currentUser = null;
const pagePaths = { dashboard: '/dashboard', notes: '/notes', posts: '/posts', files: '/files' };
let notes = [];
let files = [];
let posts = [];
let activeNoteID = null;
let activePostID = null;
let postFilter = 'all';
let saveTimer = null;
let retryTimer = null;
let retryDelay = 1000;
let toastTimer = null;
let savedRange = null;
let busySaving = false;
let saveAgain = false;

function parseTagsInput(value) {
	return [...new Set(value.split(',').map((tag) => tag.trim().toLowerCase()).filter(Boolean))];
}
