require '/etc/nginx/mruby/common.rb'
Nginx.return -> do
  return run(false)
end.call
