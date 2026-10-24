// The single source of truth for what the simulator offers. The /sim catalog,
// <sim-player> and the tests all read this file. To add a concept: add an
// entry, write its modules, and flip `status` to 'available'.

export const topics = [
  { id: 'algorithms', title: 'Algorithms' },
  { id: 'data-structures', title: 'Data structures' },
  { id: 'concurrency', title: 'Concurrency' },
  { id: 'security', title: 'Security' },
  { id: 'operations', title: 'Operations' },
  { id: 'infrastructure', title: 'Infrastructure' },
];

const SORTING = '/static/sim/concepts/algorithms/sorting';

const sorting = (id, title, file) => ({
  id,
  topic: 'algorithms',
  title,
  status: 'available',
  trace: `${SORTING}/${file}.js`,
  renderer: `${SORTING}/bars-renderer.js`,
});

const planned = (topic, id, title) => ({ id, topic, title, status: 'planned' });

export const concepts = [
  sorting('bubble-sort', 'Bubble sort', 'bubble'),
  sorting('insertion-sort', 'Insertion sort', 'insertion'),
  sorting('merge-sort', 'Merge sort', 'merge'),
  sorting('quick-sort', 'Quick sort', 'quick'),
  planned('algorithms', 'breadth-first-search', 'Breadth-first search'),
  planned('algorithms', 'depth-first-search', 'Depth-first search'),
  planned('algorithms', 'dijkstra', "Dijkstra's shortest path"),
  planned('algorithms', 'binary-search', 'Binary search'),
  planned('data-structures', 'binary-search-tree', 'Binary search tree'),
  planned('data-structures', 'hash-table', 'Hash table'),
  planned('data-structures', 'heap', 'Heap'),
  planned('concurrency', 'race-condition', 'Race condition'),
  planned('concurrency', 'deadlock', 'Deadlock'),
  planned('security', 'password-hashing', 'Password hashing'),
  planned('security', 'sql-injection', 'SQL injection'),
  planned('operations', 'circuit-breaker', 'Circuit breaker'),
  planned('operations', 'rolling-deploy', 'Rolling deploy'),
  planned('infrastructure', 'dns-resolution', 'DNS resolution'),
  planned('infrastructure', 'load-balancing', 'Load balancing'),
];

// groupByTopic nests concepts under their topic, keeping the topic order.
export function groupByTopic(topicList = topics, conceptList = concepts) {
  return topicList.map((topic) => ({
    ...topic,
    concepts: conceptList.filter((c) => c.topic === topic.id),
  }));
}
