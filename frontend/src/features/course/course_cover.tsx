import Image from 'next/image';

const TOPIC_COVERS = new Map([
  ['storyboard', '/design/storyboard.svg'],
  ['character-design', '/design/character.svg'],
  ['sound-editing', '/design/sound.svg'],
]);

export function CourseCover({
  category,
  coverUrl,
}: {
  category?: string;
  coverUrl?: string;
}) {
  const url = coverUrl?.trim();
  const source =
    url && /^(https?:\/\/|\/(?!\/))/.test(url)
      ? url
      : TOPIC_COVERS.get(category ?? '');
  if (!source) return null;

  return (
    <Image
      src={source}
      alt=""
      width={360}
      height={162}
      unoptimized
      className="aspect-[20/9] w-full object-cover"
    />
  );
}
