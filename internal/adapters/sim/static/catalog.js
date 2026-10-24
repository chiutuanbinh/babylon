import { groupByTopic } from './registry.js';

const groups = groupByTopic();
const all = groups.flatMap((g) => g.concepts);
const live = all.filter((c) => c.status === 'available').length;
document.getElementById('summary').textContent = `${live} of ${all.length} concepts available; the rest are planned.`;

const root = document.getElementById('catalog');
for (const group of groups) {
  const section = document.createElement('section');
  const heading = document.createElement('h2');
  heading.textContent = group.title;
  const list = document.createElement('ul');
  for (const c of group.concepts) {
    const item = document.createElement('li');
    if (c.status === 'available') {
      const link = document.createElement('a');
      link.href = `/sim/${c.topic}/${c.id}`;
      link.textContent = c.title;
      item.append(link);
    } else {
      item.className = 'planned';
      item.textContent = `${c.title} (planned)`;
    }
    list.append(item);
  }
  if (!group.concepts.some((c) => c.status === 'available')) section.className = 'planned';
  section.append(heading, list);
  root.append(section);
}
