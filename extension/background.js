// background.js

// Configuration
const NATIVE_HOST_NAME = "com.github.browser_pipe"; // Must match manifest.json

let port = null;

function connect() {
  console.log("Connecting to native host...");
  port = chrome.runtime.connectNative(NATIVE_HOST_NAME);

  port.onMessage.addListener((response) => {
    console.log("Received from Plumber:", response);

    if (chrome.notifications) {
      chrome.notifications.create({
        type: 'basic',
        iconUrl: 'icon.png',
        title: response.status === 'success' ? 'Browser Pipe' : 'Error',
        message: response.message
      });
    }
  });

  port.onDisconnect.addListener(() => {
    console.error("Disconnected from Plumber", chrome.runtime.lastError);
    port = null;
  });
}

function sendEnvelope(target, url, origin) {
  if (!port) {
    connect();
  }

  const envelope = {
    id: crypto.randomUUID(),
    origin: origin || "chrome", // This should be dynamic based on the browser, but hard to detect in standard WebExt API easily without distinct builds. Defaulting to 'chrome' or 'browser'.
    url: url,
    target: target || "", // Empty means use routing rules
    timestamp: Math.floor(Date.now() / 1000)
  };

  console.log("Sending envelope:", envelope);

  try {
    port.postMessage(envelope);
  } catch (e) {
    console.error("Failed to send message:", e);
  }
}

// 1. Context Menus
chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({
    id: "send-to-pipe",
    title: "Send to Browser Pipe",
    contexts: ["link", "page"]
  });
});

chrome.contextMenus.onClicked.addListener((info, tab) => {
  const url = info.linkUrl || info.pageUrl;
  // Send with empty target to let Plumber routing decide
  sendEnvelope("", url, "chrome");
});
chrome.action.onClicked.addListener((tab) => {
  if (tab.url && !tab.url.startsWith("chrome://")) {
    sendEnvelope("toggle", tab.url, "chrome");
  }
});

// Initial Connection
connect();
