def run(enable)
  begin
    r = Nginx::Request.new
    url = "/queues/#{r.var.host}"
    url << "/enable" if enable
    Nginx::Async::HTTP.sub_request url
    res = Nginx::Async::HTTP.last_response
    ho = r.headers_out
    ho["Set-Cookie"] = res.headers["Set-Cookie"]
    case res.status
    when 200
      return Nginx::DECLINED
    when 429
      r = JSON::parse(res.body)
      %w(
        serial_no
        permitted_no
      ).each do |n|
        ho[n] = r[n].to_s
      end
      return Nginx::HTTP_SERVICE_UNAVAILABLE
    end
  rescue => e
    Nginx.errlogger Nginx::LOG_ERR, e.inspect
    Nginx.errlogger Nginx::LOG_ERR, e.backtrace.join
    return Nginx::HTTP_SERVICE_UNAVAILABLE
  end
end
