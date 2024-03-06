export function getProps<T extends Record<string, any>>(
  rawProps: T,
  casts: Record<string, any>
): T {
  const validProps = Object.keys(rawProps).filter((key) => {
    return Boolean(casts[key]);
  });

  const props = {};

  validProps.forEach((key) => {
    if (casts[key] === 'string') {
      props[key] = rawProps[key];
    } else {
      props[key] = new casts[key](rawProps[key]);
    }
  });

  return props as T;
}
