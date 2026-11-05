-- Frame-only lines have no lens price to confirm. Lines saved before this fix were marked open,
-- which showed every frame order as "lens price pending" and blocked confirming it.
UPDATE order_items SET lens_price_final = true WHERE lens_kind = 'none' AND NOT lens_price_final;
