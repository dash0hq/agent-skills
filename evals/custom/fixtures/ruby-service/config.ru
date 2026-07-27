# Eval fixture: deliberately uninstrumented Ruby HTTP service (see
# evals/fixtures/README.md for the contract). It serves GET /checkout,
# performs one outbound HTTP call to DOWNSTREAM_URL while handling it, and
# uses only obviously synthetic data (user@example.test, TEST-0001). The
# listen port comes from PORT via the rackup invocation in the Dockerfile.
require 'json'
require 'net/http'
require 'uri'

class CheckoutApp
  def call(env)
    request = Rack::Request.new(env)
    unless request.get? && request.path == '/checkout'
      return [404, { 'content-type' => 'application/json' }, ['{"error":"not found"}']]
    end

    downstream_url = ENV.fetch('DOWNSTREAM_URL', 'http://localhost:9090/inventory')
    begin
      inventory = Net::HTTP.get(URI(downstream_url))
    rescue StandardError => e
      warn "inventory lookup failed: #{e}"
      return [502, { 'content-type' => 'application/json' }, ['{"error":"inventory unavailable"}']]
    end

    puts 'checkout completed order.id=TEST-0001 customer.email=user@example.test'
    body = JSON.generate(
      order_id: 'TEST-0001',
      customer_email: 'user@example.test',
      status: 'confirmed',
      inventory: inventory
    )
    [200, { 'content-type' => 'application/json' }, [body]]
  end
end

run CheckoutApp.new
