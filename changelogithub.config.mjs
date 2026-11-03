const rootReleaseTag = /^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/

export default {
  emoji: false,
  tagFilter: (tag) => rootReleaseTag.test(tag),
}
