import assert from "node:assert/strict";
import test from "node:test";

import { homeMotionState, shouldRunMotion } from "./app.js";

test("home motion runs only while visible, intersecting, and unrestricted", () => {
  assert.equal(shouldRunMotion({ reducedMotion: false, intersecting: true, pageVisible: true }), true);
  assert.equal(shouldRunMotion({ reducedMotion: false, intersecting: false, pageVisible: true }), false);
  assert.equal(shouldRunMotion({ reducedMotion: false, intersecting: true, pageVisible: false }), false);
  assert.equal(shouldRunMotion({ reducedMotion: true, intersecting: true, pageVisible: true }), false);
});

test("each home motion target derives a stable visibility state", () => {
  assert.equal(homeMotionState({ reducedMotion: false, intersecting: true, pageVisible: true }), "running");
  assert.equal(homeMotionState({ reducedMotion: false, intersecting: false, pageVisible: true }), "paused");
  assert.equal(homeMotionState({ reducedMotion: false, intersecting: true, pageVisible: false }), "paused");
  assert.equal(homeMotionState({ reducedMotion: true, intersecting: true, pageVisible: true }), "reduced");
});
